package myks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHelmConfig(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		expectedError bool
		expectedCfg   HelmConfig
	}{
		{
			name: "valid Helm config",
			yamlContent: `
helm:
  buildDependencies: true
  capabilities: [ "cap1", "cap2" ]
  includeCRDs: true
  kubeVersion: "v1.20.0"
  namespace: "test-namespace"
  charts:
    - name: chart1
      buildDependencies: false
      includeCRDs: false
      namespace: "chart1-namespace"
      releaseName: "chart1-release"
`,
			expectedError: false,
			expectedCfg: HelmConfig{
				BuildDependencies: true,
				Capabilities:      []string{"cap1", "cap2"},
				IncludeCRDs:       true,
				KubeVersion:       "v1.20.0",
				Namespace:         "test-namespace",
				Charts: map[string]HelmChartOverride{
					"chart1": {
						BuildDependencies: boolPtr(false),
						IncludeCRDs:       boolPtr(false),
						Namespace:         "chart1-namespace",
						ReleaseName:       "chart1-release",
					},
				},
			},
		},
		{
			name: "valid Helm config without charts",
			yamlContent: `
helm:
  buildDependencies: true
  capabilities: [ "cap1", "cap2" ]
  includeCRDs: true
  kubeVersion: "v1.20.0"
  namespace: "test-namespace"
`,
			expectedError: false,
			expectedCfg: HelmConfig{
				BuildDependencies: true,
				Capabilities:      []string{"cap1", "cap2"},
				IncludeCRDs:       true,
				KubeVersion:       "v1.20.0",
				Namespace:         "test-namespace",
				Charts:            nil,
			},
		},
		{
			name: "valid Helm config with multiple charts",
			yamlContent: `
helm:
  buildDependencies: true
  charts:
    - name: chart1
      releaseName: chart1-release
    - name: chart2
      releaseName: chart2-release
    - name: chart3
      releaseName: chart3-release
`,
			expectedError: false,
			expectedCfg: HelmConfig{
				BuildDependencies: true,
				Capabilities:      nil,
				IncludeCRDs:       false,
				KubeVersion:       "",
				Namespace:         "",
				Charts: map[string]HelmChartOverride{
					"chart1": {
						BuildDependencies: nil,
						IncludeCRDs:       nil,
						Namespace:         "",
						ReleaseName:       "chart1-release",
					},
					"chart2": {
						BuildDependencies: nil,
						IncludeCRDs:       nil,
						Namespace:         "",
						ReleaseName:       "chart2-release",
					},
					"chart3": {
						BuildDependencies: nil,
						IncludeCRDs:       nil,
						Namespace:         "",
						ReleaseName:       "chart3-release",
					},
				},
			},
		},
		{
			name: "valid Helm config without some fields",
			yamlContent: `
helm:
  buildDependencies: true
  namespace: "test-namespace"
`,
			expectedError: false,
			expectedCfg: HelmConfig{
				BuildDependencies: true,
				Capabilities:      nil,
				IncludeCRDs:       false,
				KubeVersion:       "",
				Namespace:         "test-namespace",
				Charts:            nil,
			},
		},
		{
			name: "missing chart name",
			yamlContent: `
helm:
  buildDependencies: true
  charts:
    - namespace: "chart-namespace"
`,
			expectedError: true,
		},
		{
			name: "duplicate chart name",
			yamlContent: `
helm:
  buildDependencies: true
  charts:
    - name: "chart1"
    - name: "chart1"
`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := newHelmConfig(tt.yamlContent)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCfg, cfg)
			}
		})
	}
}

func TestGetChartConfig(t *testing.T) {
	baseCfg := HelmConfig{
		BuildDependencies: true,
		IncludeCRDs:       true,
		Namespace:         "base-namespace",
		Charts: map[string]HelmChartOverride{
			"chart1": {
				BuildDependencies: boolPtr(false),
				Namespace:         "chart1-namespace",
			},
			"chart2": {
				ReleaseName: "chart2-release",
			},
		},
	}

	tests := []struct {
		name        string
		chartName   string
		expectedCfg HelmConfig
	}{
		{
			name:      "override exists",
			chartName: "chart1",
			expectedCfg: HelmConfig{
				BuildDependencies: false,
				IncludeCRDs:       true,
				Namespace:         "chart1-namespace",
			},
		},
		{
			name:      "override exists with release name",
			chartName: "chart2",
			expectedCfg: HelmConfig{
				BuildDependencies: true,
				IncludeCRDs:       true,
				Namespace:         "base-namespace",
				ReleaseName:       "chart2-release",
			},
		},
		{
			name:      "override does not exist",
			chartName: "chart3",
			expectedCfg: HelmConfig{
				BuildDependencies: true,
				IncludeCRDs:       true,
				Namespace:         "base-namespace",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseCfg.getChartConfig(tt.chartName)
			assert.Equal(t, tt.expectedCfg, cfg)
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}
