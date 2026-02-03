package myks

import (
	"strings"
	"testing"
)

func TestValidateVendirConfig(t *testing.T) {
	validConfig := `apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
- path: charts/httpbingo
  contents:
  - path: .
    helmChart:
      name: httpbingo
      version: 0.1.1
      repository:
        url: https://example.com/charts
`

	tests := []struct {
		name        string
		config      string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid single-document config",
			config:  validConfig,
			wantErr: false,
		},
		{
			name: "multiple empty document separators",
			config: `
---
---
---
apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
- path: charts/test
  contents:
  - path: .
    helmChart:
      name: test
      version: 1.0.0
`,
			wantErr:     true,
			errContains: "multiple YAML documents",
		},
		{
			name: "two YAML documents with content",
			config: `application:
  centralForwarder:
    resources:
      requests:
        cpu: 405m
---
apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
- path: charts/test
  contents:
  - path: .
    helmChart:
      name: test
      version: 1.0.0
`,
			wantErr:     true,
			errContains: "multiple YAML documents",
		},
		{
			name: "missing apiVersion",
			config: `kind: Config
directories:
- path: charts/test
  contents:
  - path: .
    helmChart:
      name: test
      version: 1.0.0
`,
			wantErr:     true,
			errContains: "missing required field: apiVersion",
		},
		{
			name: "invalid apiVersion",
			config: `apiVersion: v1
kind: Config
directories:
- path: charts/test
  contents:
  - path: .
    helmChart:
      name: test
      version: 1.0.0
`,
			wantErr:     true,
			errContains: "invalid apiVersion",
		},
		{
			name: "missing kind",
			config: `apiVersion: vendir.k14s.io/v1alpha1
directories:
- path: charts/test
  contents:
  - path: .
    helmChart:
      name: test
      version: 1.0.0
`,
			wantErr:     true,
			errContains: "missing required field: kind",
		},
		{
			name: "invalid kind",
			config: `apiVersion: vendir.k14s.io/v1alpha1
kind: Secret
directories:
- path: charts/test
  contents:
  - path: .
    helmChart:
      name: test
      version: 1.0.0
`,
			wantErr:     true,
			errContains: "invalid kind",
		},
		{
			name: "empty directories",
			config: `apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories: []
`,
			wantErr:     true,
			errContains: "no directories defined",
		},
		{
			name: "directory without path",
			config: `apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
- contents:
  - path: .
    helmChart:
      name: test
      version: 1.0.0
`,
			wantErr:     true,
			errContains: "directory[0] missing required field: path",
		},
		{
			name: "directory without contents",
			config: `apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
- path: charts/test
  contents: []
`,
			wantErr:     true,
			errContains: "has no contents defined",
		},
		{
			name: "config with only non-vendir content (first doc not vendir)",
			config: `application:
  centralForwarder:
    resources:
      requests:
        cpu: 405m
`,
			wantErr:     true,
			errContains: "missing required field: apiVersion",
		},
		{
			name: "valid config with multiple directories",
			config: `apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
- path: charts/app1
  contents:
  - path: .
    helmChart:
      name: app1
      version: 1.0.0
- path: charts/app2
  contents:
  - path: .
    helmChart:
      name: app2
      version: 2.0.0
`,
			wantErr: false,
		},
		{
			name: "valid config with git source",
			config: `apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
- path: ytt/argocd
  contents:
  - path: .
    git:
      url: https://github.com/argoproj/argo-cd
      ref: v2.7.3
`,
			wantErr: false,
		},
		{
			name: "valid config with directory source",
			config: `apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
- path: charts/local
  contents:
  - path: .
    directory:
      path: ../lib/charts/local-chart
`,
			wantErr: false,
		},
		{
			name: "multiple documents with leading separator",
			config: `---
apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
- path: charts/test
  contents:
  - path: .
    helmChart:
      name: test
      version: 1.0.0
`,
			wantErr: false,
		},
		{
			name: "three YAML documents",
			config: `foo: bar
---
baz: qux
---
apiVersion: vendir.k14s.io/v1alpha1
kind: Config
directories:
- path: charts/test
  contents:
  - path: .
    helmChart:
      name: test
      version: 1.0.0
`,
			wantErr:     true,
			errContains: "multiple YAML documents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVendirConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVendirConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("validateVendirConfig() error = %v, expected to contain %q", err, tt.errContains)
				}
			}
		})
	}
}
