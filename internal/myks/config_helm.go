package myks

import (
	"fmt"

	yaml "gopkg.in/yaml.v3"
)

type HelmConfig struct {
	BuildDependencies bool     `yaml:"buildDependencies"`
	Capabilities      []string `yaml:"capabilities"`
	IncludeCRDs       bool     `yaml:"includeCRDs"`
	KubeVersion       string   `yaml:"kubeVersion"`
	Namespace         string   `yaml:"namespace"`
	ReleaseName       string

	Charts map[string]HelmChartOverride `yaml:"charts"`
}

type HelmChartOverride struct {
	BuildDependencies *bool  `yaml:"buildDependencies"`
	IncludeCRDs       *bool  `yaml:"includeCRDs"`
	Namespace         string `yaml:"namespace"`
	ReleaseName       string `yaml:"releaseName"`
}

func newHelmConfig(dataValuesYaml string) (HelmConfig, error) {
	type originalChartConfig struct {
		BuildDependencies *bool  `yaml:"buildDependencies"`
		IncludeCRDs       *bool  `yaml:"includeCRDs"`
		Name              string `yaml:"name"`
		Namespace         string `yaml:"namespace"`
		ReleaseName       string `yaml:"releaseName"`
	}

	type fullHelmConfig struct {
		BuildDependencies bool                  `yaml:"buildDependencies"`
		Capabilities      []string              `yaml:"capabilities"`
		IncludeCRDs       bool                  `yaml:"includeCRDs"`
		KubeVersion       string                `yaml:"kubeVersion"`
		Namespace         string                `yaml:"namespace"`
		Charts            []originalChartConfig `yaml:"charts"`
	}

	var helmConfigWrapper struct {
		Helm fullHelmConfig `yaml:"helm"`
	}

	if err := yaml.Unmarshal([]byte(dataValuesYaml), &helmConfigWrapper); err != nil {
		return HelmConfig{}, err
	}

	helmConfig := HelmConfig{
		BuildDependencies: helmConfigWrapper.Helm.BuildDependencies,
		Capabilities:      helmConfigWrapper.Helm.Capabilities,
		IncludeCRDs:       helmConfigWrapper.Helm.IncludeCRDs,
		KubeVersion:       helmConfigWrapper.Helm.KubeVersion,
		Namespace:         helmConfigWrapper.Helm.Namespace,
	}

	chartConfigs := map[string]HelmChartOverride{}
	for i, chart := range helmConfigWrapper.Helm.Charts {
		if chart.Name == "" {
			return HelmConfig{}, fmt.Errorf("helm.charts[%d].name is required", i)
		}
		if _, ok := chartConfigs[chart.Name]; ok {
			return HelmConfig{}, fmt.Errorf("helm.charts[%d].name is not unique", i)
		}
		chartConfigs[chart.Name] = HelmChartOverride{
			BuildDependencies: chart.BuildDependencies,
			IncludeCRDs:       chart.IncludeCRDs,
			Namespace:         chart.Namespace,
			ReleaseName:       chart.ReleaseName,
		}
	}

	return helmConfig, nil
}

func (cfg *HelmConfig) getChartConfig(chartName string) HelmConfig {
	chartConfig := HelmConfig{
		BuildDependencies: cfg.BuildDependencies,
		IncludeCRDs:       cfg.IncludeCRDs,
		Namespace:         cfg.Namespace,
	}

	if cc, ok := cfg.Charts[chartName]; ok {
		if cc.Namespace != "" {
			chartConfig.Namespace = cc.Namespace
		}
		if cc.ReleaseName != "" {
			chartConfig.ReleaseName = cc.ReleaseName
		}
		if cc.BuildDependencies != nil {
			chartConfig.BuildDependencies = *cc.BuildDependencies
		}
		if cc.IncludeCRDs != nil {
			chartConfig.IncludeCRDs = *cc.IncludeCRDs
		}
	}

	return chartConfig
}
