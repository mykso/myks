package myks

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
)

type HelmSyncer struct {
	ident string
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
	chartNames := []string{}
	for _, chartDir := range chartsDirs {
		chartName := filepath.Base(chartDir)
		chartNames = append(chartNames, chartName)
		chartConfig := helmConfig.getChartConfig(chartName)
		if !chartConfig.BuildDependencies {
			log.Debug().Msg(a.Msg(hr.getStepName(), fmt.Sprintf(".helm.charts[%s].buildDependencies is disabled, skipping", chartName)))
			continue
		}
		if err = hr.helmBuild(a, chartDir); err != nil {
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
	for _, dependency := range dependencies.([]interface{}) {
		depMap := dependency.(map[string]interface{})
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
