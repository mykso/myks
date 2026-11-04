package myks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootstrapScaffold(t *testing.T) {
	components := []string{"config", "environments", "gitignore", "prototypes", "schema"}
	tests := []struct {
		name       string
		components []string
		present    []string
		absent     []string
		devAppsHas string
	}{
		{
			name:       "minimal",
			components: components,
			present:    []string{"envs/dev/env-data.ytt.yaml", "envs/prod/env-data.ytt.yaml", "prototypes/httpbingo/vendir/base.ytt.yaml"},
			absent:     []string{"prototypes/argocd", "envs/_env/argocd"},
		},
		{
			name:       "argocd",
			components: append(components, "argocd"),
			present:    []string{"envs/prod/env-data.ytt.yaml", "prototypes/argocd/vendir/base.ytt.yaml", "envs/_env/argocd/secret.overlay.ytt.yaml"},
			devAppsHas: "proto: argocd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithDefaults()
			g.RootDir = t.TempDir()
			require.NoError(t, g.Bootstrap(false, false, tt.components, "v0.0.0"))

			for _, p := range tt.present {
				assert.FileExists(t, filepath.Join(g.RootDir, p))
			}
			for _, p := range tt.absent {
				assert.NoFileExists(t, filepath.Join(g.RootDir, p))
				assert.NoDirExists(t, filepath.Join(g.RootDir, p))
			}

			rootData, err := os.ReadFile(filepath.Join(g.RootDir, "envs", "env-data.ytt.yaml"))
			require.NoError(t, err)
			devData, err := os.ReadFile(filepath.Join(g.RootDir, "envs", "dev", "env-data.ytt.yaml"))
			require.NoError(t, err)
			if tt.devAppsHas == "" {
				assert.Contains(t, string(rootData), "enabled: false")
				assert.NotContains(t, string(devData), "proto: argocd")
			} else {
				assert.NotContains(t, string(rootData), "enabled: false")
				assert.Contains(t, string(devData), tt.devAppsHas)
			}
		})
	}
}

func TestGitHubWorkflowsPinVersion(t *testing.T) {
	tests := []struct {
		version string
		want    string
	}{
		{"5.16.0", "MYKS_VERSION: v5.16.0"},
		{"dev", "MYKS_VERSION: latest"},
		{"5.16.1-SNAPSHOT-abc1234", "MYKS_VERSION: latest"},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			g := NewWithDefaults()
			g.RootDir = t.TempDir()
			require.NoError(t, g.Bootstrap(false, false, []string{"ci-github"}, tt.version))
			for _, name := range []string{"render.yaml", "drift.yaml"} {
				content, err := os.ReadFile(filepath.Join(g.RootDir, ".github", "workflows", name))
				require.NoError(t, err)
				assert.Contains(t, string(content), tt.want)
				assert.NotContains(t, string(content), "__MYKS_VERSION__")
			}
		})
	}
}
