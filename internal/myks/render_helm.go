package myks

import (
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
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

	helmConfig, err := h.getHelmConfig()
	if err != nil {
		log.Warn().Err(err).Msg(h.app.Msg(helmStepName, "Unable to get helm config"))
		return "", err
	}

	var commonHelmArgs []string

	// FIXME: move Namespace to a per-chart config
	if helmConfig.Namespace == "" {
		helmConfig.Namespace = h.app.e.g.NamespacePrefix + h.app.Name
	}
	commonHelmArgs = append(commonHelmArgs, "--namespace", helmConfig.Namespace)

	if helmConfig.KubeVersion != "" {
		commonHelmArgs = append(commonHelmArgs, "--kube-version", helmConfig.KubeVersion)
	}

	// FIXME: move IncludeCRDs to a per-chart config
	if helmConfig.IncludeCRDs {
		commonHelmArgs = append(commonHelmArgs, "--include-crds")
	}

	for _, capa := range helmConfig.Capabilities {
		commonHelmArgs = append(commonHelmArgs, "--api-versions", capa)
	}
	var outputs []string

	for _, chartDir := range chartDirs {
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

		res, err := h.app.runCmd("helm template chart", "helm", nil, append(helmArgs, commonHelmArgs...))
		if err != nil {
			log.Error().Msg(h.app.Msg(helmStepName, res.Stderr))
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
		log.Warn().Err(err).Msg(h.app.Msg(helmStepName, "Unable to inspect data values"))
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
