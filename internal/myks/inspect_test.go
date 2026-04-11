package myks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/creasty/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestApp creates a minimal Application wired to a temp directory for testing.
func newTestApp(t *testing.T) *Application {
	t.Helper()
	cfg := &Config{}
	require.NoError(t, defaults.Set(cfg))
	cfg.RootDir = t.TempDir()

	env := &Environment{
		Dir: "envs/test-env",
		ID:  "test-env",
		cfg: cfg,
	}

	return &Application{
		Name:      "test-app",
		Prototype: filepath.Join(cfg.RootDir, cfg.PrototypesDir, "test-proto"),
		e:         env,
		cfg:       cfg,
	}
}

func Test_inspectRenderedArtifacts(t *testing.T) {
	t.Run("returns nil when no artifacts exist", func(t *testing.T) {
		app := newTestApp(t)
		result := app.inspectRenderedArtifacts()
		assert.Nil(t, result)
	})

	t.Run("reads vendir config", func(t *testing.T) {
		app := newTestApp(t)
		vendirPath := app.expandServicePath(app.cfg.VendirConfigFileName)
		require.NoError(t, os.MkdirAll(filepath.Dir(vendirPath), 0o700))
		require.NoError(t, os.WriteFile(vendirPath, []byte("vendir-content"), 0o600))

		result := app.inspectRenderedArtifacts()
		require.NotNil(t, result)
		assert.Equal(t, "vendir-content", result.VendirConfig)
		assert.Nil(t, result.HelmValues)
		assert.Nil(t, result.StepOutputs)
	})

	t.Run("reads helm values and skips unmerged and non-yaml files", func(t *testing.T) {
		app := newTestApp(t)
		helmDir := app.expandServicePath(app.cfg.HelmStepDirName)
		require.NoError(t, os.MkdirAll(helmDir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(helmDir, "values.yaml"), []byte("key: val"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(helmDir, "unmerged_values.yaml"), []byte("skip"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(helmDir, "notes.txt"), []byte("skip"), 0o600))

		result := app.inspectRenderedArtifacts()
		require.NotNil(t, result)
		assert.Equal(t, map[string]string{"values.yaml": "key: val"}, result.HelmValues)
	})

	t.Run("reads step outputs", func(t *testing.T) {
		app := newTestApp(t)
		stepsDir := app.expandServicePath("steps")
		require.NoError(t, os.MkdirAll(stepsDir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(stepsDir, "render-ytt.yaml"), []byte("output"), 0o600))

		result := app.inspectRenderedArtifacts()
		require.NotNil(t, result)
		assert.Equal(t, map[string]string{"render-ytt.yaml": "output"}, result.StepOutputs)
	})

	t.Run("skips subdirectories in helm dir", func(t *testing.T) {
		app := newTestApp(t)
		helmDir := app.expandServicePath(app.cfg.HelmStepDirName)
		require.NoError(t, os.MkdirAll(filepath.Join(helmDir, "subdir"), 0o700))

		result := app.inspectRenderedArtifacts()
		assert.Nil(t, result)
	})
}

func Test_isInternalDir(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want bool
	}{
		{name: "underscore prefix is internal", dir: "_vendir", want: true},
		{name: "underscore only is internal", dir: "_", want: true},
		{name: "normal name is not internal", dir: "prototypes", want: false},
		{name: "empty string is not internal", dir: "", want: false},
		{name: "leading dot is not internal", dir: ".myks", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isInternalDir(tt.dir))
		})
	}
}
