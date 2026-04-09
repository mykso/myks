package myks

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mykso/myks/internal/locker"
	"github.com/rs/zerolog/log"
)

// HelmSyncer is responsible for building Helm charts' dependencies.
type HelmSyncer struct {
	ident  string
	locker *locker.Locker
	// builtCharts tracks helm dependency builds in-flight or completed this run.
	// Key: cache name, Value: *syncResult
	// Used to ensure each chart's dependencies are built at most once per run.
	builtCharts sync.Map

	// Dedup counters for observability
	BuildExecuted atomic.Int64
	BuildSkipped  atomic.Int64
}

// NewHelmSyncer creates a new HelmSyncer with the given locker.
func NewHelmSyncer(lock *locker.Locker) *HelmSyncer {
	return &HelmSyncer{
		ident:  "helm",
		locker: lock,
	}
}

func (hr *HelmSyncer) Ident() string {
	return hr.ident
}

func (hr *HelmSyncer) GenerateSecrets(_ *Globe) (string, error) {
	return "", nil
}

func (hr *HelmSyncer) Sync(a *Application, _ string) error {
	helmConfig, err := hr.getHelmConfig(a)
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(hr.getStepName(), "Unable to get helm config"))
		return err
	}

	chartsDirs, err := a.getHelmChartsDirs(hr.getStepName())
	if err != nil {
		return err
	}

	linksMap, err := a.getLinksMap()
	if err != nil {
		return err
	}

	chartNames := []string{}
	for _, chartDir := range chartsDirs {
		chartName := filepath.Base(chartDir)
		chartNames = append(chartNames, chartName)
		chartConfig := helmConfig.getChartConfig(chartName)
		if !chartConfig.BuildDependencies {
			log.Debug().Msg(a.Msg(hr.getStepName(), fmt.Sprintf(".helm.charts[%s].buildDependencies is disabled, skipping", chartName)))
			continue
		}
		cacheName := findCacheNameForChart(linksMap, chartDir, a.cfg.HelmChartsDirName)
		if err := hr.buildChartOnce(a, cacheName, chartDir); err != nil {
			return err
		}
	}
	for chart := range helmConfig.Charts {
		if !slices.Contains(chartNames, chart) {
			log.Warn().Msg(a.Msg(hr.getStepName(), fmt.Sprintf("'%s' chart defined in .helm.charts is not found in the charts directory", chart)))
		}
	}
	log.Info().Msg(a.Msg(hr.getStepName(), "Synced"))
	return nil
}

// findCacheNameForChart looks up the vendir cache name for a given chart directory by
// searching the links map. The chart directory base name (e.g. "nginx") combined with
// the helm charts dir name (e.g. "charts") forms the vendor path key (e.g. "charts/nginx").
func findCacheNameForChart(linksMap map[string]string, chartDir, helmChartsDirName string) string {
	vendorPath := filepath.Join(helmChartsDirName, filepath.Base(chartDir))
	return linksMap[vendorPath]
}

// buildChartOnce ensures helm dependencies are built at most once per chart cache entry per run.
// The first goroutine per cacheName acquires the write lock and runs helm dependencies build;
// subsequent goroutines wait for that result and skip the redundant build.
// When cacheName is empty (chart not in the links map), falls back to a per-chart-dir key.
// This deduplicates concurrent builds of the same chart directory during the current run.
func (hr *HelmSyncer) buildChartOnce(a *Application, cacheName, chartDir string) error {
	key := cacheName
	if key == "" {
		// Fallback for charts not tracked via vendir cache (e.g. checked-in charts).
		// Use the chart dir path as the dedup key so concurrent builds of the same dir are deduplicated.
		log.Debug().Str("chart", filepath.Base(chartDir)).Msg(a.Msg(hr.getStepName(), "Chart not found in links map, using chart dir as dedup key"))
		key = chartDir
	}

	result := &syncResult{done: make(chan struct{})}
	if existing, loaded := hr.builtCharts.LoadOrStore(key, result); loaded {
		existingResult := existing.(*syncResult)
		<-existingResult.done
		hr.BuildSkipped.Add(1)
		log.Debug().Str("cache", cacheName).Msg(a.Msg(hr.getStepName(), "Skipped helm dependencies build (already built this run)"))
		return existingResult.err
	}
	defer close(result.done)

	// Only the singleflight winner acquires the write lock and builds.
	// The write lock coordinates with the render phase's read lock on the same cache name,
	// preventing reads of partially-built chart data during helm dependency download.
	unlock := hr.locker.LockNames(slices.Values([]string{key}), true)
	defer unlock()

	hr.BuildExecuted.Add(1)
	result.err = hr.helmBuild(a, chartDir)
	return result.err
}

// GetDedupStats returns a snapshot of the helm build dedup counters.
func (hr *HelmSyncer) GetDedupStats() *HelmDedupStats {
	return &HelmDedupStats{
		Executed: hr.BuildExecuted.Load(),
		Skipped:  hr.BuildSkipped.Load(),
	}
}

func (hr *HelmSyncer) helmBuild(a *Application, chartDir string) error {
	chartPath := filepath.Join(chartDir, "Chart.yaml")
	if exists, _ := isExist(chartDir); !exists {
		return fmt.Errorf("can't locate Chart.yaml at: %s", chartPath)
	}

	chart, err := unmarshalYamlToMap(chartPath)
	if err != nil {
		return fmt.Errorf("failure to unmarshal Chart.yaml at: %s", chartPath)
	}

	dependencies, ok := chart["dependencies"]
	if !ok {
		return nil
	}

	helmCache := a.expandServicePath("helm-cache")
	cacheArgs := []string{
		"--repository-cache", filepath.Join(helmCache, "repository"),
		"--repository-config", filepath.Join(helmCache, "repositories.yaml"),
	}
	for _, dependency := range dependencies.([]any) {
		depMap := dependency.(map[string]any)
		repo := depMap["repository"].(string)
		if strings.HasPrefix(repo, "http") {
			args := []string{"repo", "add", createURLSlug(repo), repo, "--force-update"}
			_, err = a.runCmd(hr.getStepName(), "helm repo add", "helm", nil, append(args, cacheArgs...))
			if err != nil {
				return fmt.Errorf("failed to add repository %s in %s ", repo, chartPath)
			}
		}
	}

	buildArgs := []string{"dependencies", "build", chartDir, "--skip-refresh"}
	_, err = a.runCmd(hr.getStepName(), "helm dependencies build", "helm", nil, append(buildArgs, cacheArgs...))
	if err != nil {
		return fmt.Errorf("failed to build dependencies for chart %s", chartDir)
	}
	return nil
}

func (hr *HelmSyncer) getHelmConfig(a *Application) (HelmConfig, error) {
	dataValuesYaml, err := a.ytt(hr.getStepName(), "get helm config", a.yttDataFiles, "--data-values-inspect")
	if err != nil {
		return HelmConfig{}, err
	}

	return newHelmConfig(dataValuesYaml.Stdout)
}

func (hr *HelmSyncer) getStepName() string {
	return fmt.Sprintf("%s-%s", syncStepName, hr.Ident())
}
