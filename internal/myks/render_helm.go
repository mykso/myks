package myks

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/mykso/myks/internal/locker"
)

type Helm struct {
	additive bool
	app      *Application
	ident    string
	locker   *locker.Locker
}

// NewHelmRenderer creates a new Helm renderer for the given application and locker.
func NewHelmRenderer(app *Application, lock *locker.Locker) *Helm {
	return &Helm{
		additive: true,
		app:      app,
		ident:    "helm",
		locker:   lock,
	}
}

// AcquireLock acquires a read lock on the Helm charts vendor directory for this application.
func (h *Helm) AcquireLock() (func(), error) {
	return h.app.AcquireRenderLock(h.locker, func(path string) bool {
		return strings.HasPrefix(path, h.app.cfg.HelmChartsDirName+"/")
	}, false)
}

func (h *Helm) IsAdditive() bool {
	return h.additive
}

func (h *Helm) Ident() string {
	return h.ident
}

func (h *Helm) Render(_ string) (string, error) {
	log.Debug().Msg(h.app.Msg(h.getStepName(), "Starting"))
	outputs := []string{}

	chartsDirs, err := h.app.getHelmChartsDirs(h.getStepName())
	if err != nil {
		return "", err
	}

	helmConfig, err := h.getHelmConfig()
	if err != nil {
		log.Warn().Err(err).Msg(h.app.Msg(h.getStepName(), "Unable to get helm config"))
		return "", err
	}

	var commonHelmArgs []string

	if helmConfig.KubeVersion != "" {
		commonHelmArgs = append(commonHelmArgs, "--kube-version", helmConfig.KubeVersion)
	}

	for _, capa := range helmConfig.Capabilities {
		commonHelmArgs = append(commonHelmArgs, "--api-versions", capa)
	}

	chartNames := []string{}
	for _, chartDir := range chartsDirs {
		chartName := filepath.Base(chartDir)
		chartNames = append(chartNames, chartName)
		chartConfig := helmConfig.getChartConfig(chartName)
		var helmValuesFile string
		if helmValuesFile, err = h.app.prepareValuesFile(h.app.cfg.HelmStepDirName, chartName); err != nil {
			log.Warn().Err(err).Msg(h.app.Msg(h.getStepName(), "Unable to prepare helm values"))
			return "", err
		}

		if chartConfig.ReleaseName == "" {
			chartConfig.ReleaseName = chartName
		}

		helmArgs := []string{
			"template",
			"--skip-tests",
			chartConfig.ReleaseName,
			chartDir,
		}

		if chartConfig.Namespace == "" {
			chartConfig.Namespace = h.app.Name
		}
		helmArgs = append(helmArgs, "--namespace", chartConfig.Namespace)

		if chartConfig.IncludeCRDs {
			helmArgs = append(helmArgs, "--include-crds")
		}

		if helmValuesFile != "" {
			helmArgs = append(helmArgs, "--values", helmValuesFile)
		}

		res, err := h.app.runCmd(h.getStepName(), "helm template chart", "helm", nil, append(helmArgs, commonHelmArgs...))
		if err != nil {
			return "", err
		}

		if res.Stdout == "" {
			log.Warn().Str("chart", chartName).Msg(h.app.Msg(h.getStepName(), "No helm output"))
			continue
		}

		outputs = append(outputs, res.Stdout)
	}

	h.warnOnOrphanConfigs(&helmConfig, chartNames)

	log.Info().Msg(h.app.Msg(h.getStepName(), "Helm chart rendered"))
	return strings.Join(outputs, "---\n"), nil
}

// warnOnOrphanConfigs checks if there are any Helm configs or values files for non-existing charts
func (h *Helm) warnOnOrphanConfigs(helmConfig *HelmConfig, charts []string) {
	for chartName := range helmConfig.Charts {
		if !slices.Contains(charts, chartName) {
			log.Warn().Msg(h.app.Msg(h.getStepName(), fmt.Sprintf("'%s' chart defined in .helm.charts is not found in the charts directory", chartName)))
		}
	}

	allValuesFiles := h.app.collectAllFilesByGlob(filepath.Join(h.app.cfg.HelmStepDirName, "*.yaml"))

	for _, valuesFile := range allValuesFiles {
		chartName, _, _ := strings.Cut(filepath.Base(valuesFile), ".")
		if chartName == "_global" {
			continue
		}
		if !slices.Contains(charts, chartName) {
			log.Warn().Msg(h.app.Msg(h.getStepName(), fmt.Sprintf("'%s' values file doesn't belong to any chart", valuesFile)))
		}
	}
}

// helmValuesSourceFiles returns all helm values source files for this application.
// It collects global values (_global.*yaml) and per-chart values (*.*yaml) from:
//   - prototypes/<prototype>/helm/
//   - envs/**/_proto/<prototype>/helm/ (at each env hierarchy level)
//   - envs/**/_apps/<app>/helm/ (at each env hierarchy level)
//
// This is a broad collection of all helm config files; during rendering each
// chart uses only the files matching its own name via prepareValuesFile.
// Used by both helm render and inspect.
func (a *Application) helmValuesSourceFiles() []string {
	return a.collectAllFilesByGlob(filepath.Join(a.cfg.HelmStepDirName, "*.*yaml"))
}

func (h *Helm) getHelmConfig() (HelmConfig, error) {
	dataValuesYaml, err := h.app.ytt(h.getStepName(), "get helm config", h.app.yttDataFiles, "--data-values-inspect")
	if err != nil {
		return HelmConfig{}, err
	}

	return newHelmConfig(dataValuesYaml.Stdout)
}

func (h *Helm) getStepName() string {
	return fmt.Sprintf("%s-%s", renderStepName, h.Ident())
}
