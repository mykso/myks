package myks

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
)

type Helm struct {
	ident    string
	app      *Application
	additive bool
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
		if helmValuesFile, err = h.app.prepareValuesFile(h.app.e.g.HelmStepDirName, chartName); err != nil {
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

	h.warnOnOrphanConfigs(helmConfig, chartNames)

	log.Info().Msg(h.app.Msg(h.getStepName(), "Helm chart rendered"))
	return strings.Join(outputs, "---\n"), nil
}

// warnOnOrphanConfigs checks if there are any Helm configs or values files for non-existing charts
func (h *Helm) warnOnOrphanConfigs(helmConfig HelmConfig, charts []string) {
	for chartName := range helmConfig.Charts {
		if !slices.Contains(charts, chartName) {
			log.Warn().Msg(h.app.Msg(h.getStepName(), fmt.Sprintf("'%s' chart defined in .helm.charts is not found in the charts directory", chartName)))
		}
	}

	allValuesFiles := h.app.collectAllFilesByGlob(filepath.Join(h.app.e.g.HelmStepDirName, "*.yaml"))

	for _, valuesFile := range allValuesFiles {
		chartName := strings.SplitN(filepath.Base(valuesFile), ".", 2)[0]
		if chartName == "_global" {
			continue
		}
		if !slices.Contains(charts, chartName) {
			log.Warn().Msg(h.app.Msg(h.getStepName(), fmt.Sprintf("'%s' values file doesn't belong to any chart", valuesFile)))
		}
	}
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
