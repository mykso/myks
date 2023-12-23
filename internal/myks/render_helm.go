package myks

import (
	"fmt"
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
			err = h.helmBuild(chartDir)
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

func (h *Helm) helmBuild(chartDir string) error {
	chartPath := filepath.Join(chartDir, "Chart.yaml")
	if exists, _ := isExist(chartDir); exists == false {
		return fmt.Errorf("can't locate Chart.yaml at: %s", chartPath)
	}

	chart, err := unmarshalYamlToMap(chartPath)
	if err != nil {
		return fmt.Errorf("failure to unmarshal Chart.yaml at: %s", chartPath)
	}

	helmCache := h.app.expandTempPath("helm-cache")
	cacheArgs := []string{
		"--repository-cache", filepath.Join(helmCache, "repository"),
		"--repository-config", filepath.Join(helmCache, "repositories.yaml"),
	}
	dependencies := chart["dependencies"].([]interface{})
	for _, dependency := range dependencies {
		depMap := dependency.(map[string]interface{})
		repo := depMap["repository"].(string)
		if strings.HasPrefix(repo, "http") {
			args := []string{"repo", "add", createURLSlug(repo), repo}
			_, err := h.app.runCmd(helmStepName, "helm repo add", "helm", nil, append(args, cacheArgs...))
			if err != nil {
				return fmt.Errorf("failed to add repository %s in %s ", repo, chartPath)
			}
		}
	}

	buildArgs := []string{"dependencies", "build", chartDir}
	_, err = h.app.runCmd(helmStepName, "helm dependencies build", "helm", nil, append(buildArgs, cacheArgs...))
	if err != nil {
		return fmt.Errorf("failed to build dependencies for chart %s", chartDir)
	}
	return nil
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
