package myks

import (
	"gopkg.in/yaml.v3"
	"path/filepath"
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
	chartDir, err := h.app.getVendoredDir(h.app.e.g.HelmChartsDirName)
	if err != nil {
		log.Err(err).Msg(h.app.Msg(helmStepName, "Unable to get helm charts dir"))
		return "", err
	}

	if chartDir == "" {
		log.Debug().Msg(h.app.Msg(helmStepName, "No Helm charts found"))
		return "", nil
	}

	chartDirs, err := getSubDirs(chartDir)
	if err != nil {
		log.Err(err).Msg(h.app.Msg(helmStepName, "Unable to get helm charts sub dirs"))
		return "", err
	}
	if len(chartDirs) == 0 {
		log.Debug().Msg(h.app.Msg(helmStepName, "No Helm charts found"))
		return "", nil
	}

	var commonHelmArgs []string

	// FIXME: move Namespace to a per-chart config
	if h.app.HelmConfig.Namespace == "" {
		h.app.HelmConfig.Namespace = h.app.e.g.NamespacePrefix + h.app.Name
	}
	commonHelmArgs = append(commonHelmArgs, "--namespace", h.app.HelmConfig.Namespace)

	if h.app.HelmConfig.KubeVersion != "" {
		commonHelmArgs = append(commonHelmArgs, "--kube-version", h.app.HelmConfig.KubeVersion)
	}

	// FIXME: move IncludeCRDs to a per-chart config
	if h.app.HelmConfig.IncludeCRDs {
		commonHelmArgs = append(commonHelmArgs, "--include-crds")
	}

	for _, capa := range h.app.HelmConfig.Capabilities {
		commonHelmArgs = append(commonHelmArgs, "--api-versions", capa)
	}
	var outputs []string

	for _, chartDir := range chartDirs {

		if h.app.HelmConfig.BuildDependencies {
			_, err := h.app.runCmd(helmStepName, "helm dependencies build", "helm", nil, []string{"dependencies", "build", chartDir, "--skip-refresh"})
			if err != nil {
				return "", err
			}
		}

		chartName := filepath.Base(chartDir)
		var helmValuesFile string
		if helmValuesFile, err = h.app.prepareValuesFile("helm", chartName); err != nil {
			log.Warn().Err(err).Msg(h.app.Msg(helmStepName, "Unable to prepare helm values"))
			return "", err
		}

		// FIXME: replace h.app.Name with a name of the chart being processed
		helmArgs := []string{
			"template",
			"--skip-tests",
			chartName,
			chartDir,
		}

		if helmValuesFile != "" {
			helmArgs = append(helmArgs, "--values", helmValuesFile)
		}

		res, err := h.app.runCmd(helmStepName, "helm template chart", "helm", nil, append(helmArgs, commonHelmArgs...))
		if err != nil {
			return "", err
		}

		if res.Stdout == "" {
			log.Warn().Str("chart", chartName).Msg(h.app.Msg(helmStepName, "No helm output"))
			continue
		}

		outputs = append(outputs, res.Stdout)

	}

	log.Info().Msg(h.app.Msg(helmStepName, "Helm chart rendered"))

	return strings.Join(outputs, "---\n"), nil
}

func (h *Helm) getHelmConfig() (HelmConfig, error) {
	dataValuesYaml, err := h.app.ytt(helmStepName, "get helm config", h.app.yttDataFiles, "--data-values-inspect")
	if err != nil {
		return HelmConfig{}, err
	}

	var helmConfig struct {
		Helm HelmConfig
	}
	err = yaml.Unmarshal([]byte(dataValuesYaml.Stdout), &helmConfig)
	if err != nil {
		log.Warn().Err(err).Msg(h.app.Msg(helmStepName, "Unable to unmarshal data values"))
		return HelmConfig{}, err
	}

	return helmConfig.Helm, nil
}
