package myks

import (
	"fmt"
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
	log.Debug().Msg(h.app.Msg(h.getStepName(), "Starting"))
	vendirConfigPath := h.app.expandServicePath(h.app.e.g.VendirPatchedConfigFileName)
	// read vendir config
	vendirConfig, err := unmarshalYamlToMap(vendirConfigPath)
	if len(vendirConfig) == 0 || err != nil {
		return "", err
	}
	var outputs []string
	for _, dir := range vendirConfig["directories"].([]interface{}) {
		dirMap := dir.(map[string]interface{})
		config := make(map[string]interface{})
		dirPath := dirMap["path"].(string)
		config["path"] = dirPath
		for _, content := range dirMap["contents"].([]interface{}) {
			contentMap := content.(map[string]interface{})
			path := filepath.Join(dirPath, contentMap["path"].(string))

			// locate charts subpath in path
			chartDir, found := findSubPath(path, h.app.e.g.HelmChartsDirName)

			if !found {
				log.Debug().Msg(h.app.Msg(h.getStepName(), "No Helm charts found"))
				continue
			}

			exists, err := isExist(chartDir)
			if err != nil {
				log.Err(err).Msg(h.app.Msg(h.getStepName(), "Unable to get helm charts dir"))
				return "", err
			}

			if !exists {
				log.Debug().Msg(h.app.Msg(h.getStepName(), "No Helm charts found"))
				continue
			}

			chartDirs, err := getSubDirs(chartDir)
			if err != nil {
				log.Err(err).Msg(h.app.Msg(h.getStepName(), "Unable to get helm charts sub dirs"))
				return "", err
			}

			if len(chartDirs) == 0 {
				log.Debug().Msg(h.app.Msg(h.getStepName(), "No Helm charts found"))
				return "", nil
			}

			helmConfig, err := h.getHelmConfig()
			if err != nil {
				log.Warn().Err(err).Msg(h.app.Msg(h.getStepName(), "Unable to get helm config"))
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

			for _, chartDir := range chartDirs {

				chartName := filepath.Base(chartDir)
				var helmValuesFile string
				if helmValuesFile, err = h.app.prepareValuesFile("helm", chartName); err != nil {
					log.Warn().Err(err).Msg(h.app.Msg(h.getStepName(), "Unable to prepare helm values"))
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
		}
	}

	log.Info().Msg(h.app.Msg(h.getStepName(), "Helm chart rendered"))

	return strings.Join(outputs, "---\n"), nil
}

func (h *Helm) getHelmConfig() (HelmConfig, error) {
	dataValuesYaml, err := h.app.ytt(h.getStepName(), "get helm config", h.app.yttDataFiles, "--data-values-inspect")
	if err != nil {
		return HelmConfig{}, err
	}

	var helmConfig struct {
		Helm HelmConfig
	}
	err = yaml.Unmarshal([]byte(dataValuesYaml.Stdout), &helmConfig)
	if err != nil {
		log.Warn().Err(err).Msg(h.app.Msg(h.getStepName(), "Unable to unmarshal data values"))
		return HelmConfig{}, err
	}

	return helmConfig.Helm, nil
}

func (h *Helm) getStepName() string {
	return fmt.Sprintf("%s-%s", renderStepName, h.Ident())
}
