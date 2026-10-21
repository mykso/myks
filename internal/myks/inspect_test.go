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
		Dir: "envs/" + testEnvID,
		ID:  testEnvID,
		cfg: cfg,
	}

	return &Application{
		Name:      testAppName,
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
		{name: "leading dot is not internal", dir: testServiceDir, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isInternalDir(tt.dir))
		})
	}
}

func Test_collectFiles_malformedPattern(t *testing.T) {
	app := newTestApp(t)

	_, err := app.collectAllFilesByGlob("helm/[")
	require.ErrorIs(t, err, filepath.ErrBadPattern)

	_, err = app.e.collectBySubpath("env-data[.yaml")
	require.ErrorIs(t, err, filepath.ErrBadPattern)

	app.cfg.HelmStepDirName = "helm["
	_, err = app.inspectStepFiles()
	require.ErrorIs(t, err, filepath.ErrBadPattern)
}

func Test_inspectOverrides(t *testing.T) {
	app := newTestApp(t)
	app.e.extraYttPaths = []string{"./assets/data-schema.ytt.yaml"}
	write := func(rel, content string) string {
		path := filepath.Join(app.cfg.RootDir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}
	schema := write("prototypes/test-proto/app-data.schema.yaml", "#@data/values-schema\n---\n#@overlay/match-child-defaults missing_ok=True\napplication:\n  name: \"\"\n  version: \"1.0\"\n  hosts: [\"\"]\n")
	proto := write("prototypes/test-proto/app-data.yaml", "#@data/values\n---\napplication:\n  name: #@ \"app\"\n  version: \"1.0\"\n")
	env := write("envs/test-env/_apps/test-app/app-data.yaml", "#@data/values\n---\napplication:\n  version: \"2.0\"\n  hosts: [a]\n  extra: x\n")
	again := write("envs/test-env/_apps/test-app/app-data.again.yaml", "#@data/values\n---\napplication:\n  #@overlay/replace\n  hosts: [a]\n  name: app\n")
	vendir := write("prototypes/test-proto/vendir/values.yaml", "#@data/values\n---\napplication:\n  version: \"2.0\"\n")
	// A schema applies before every data value, even one in an earlier file.
	write("prototypes/test-proto/vendir/schema.yaml", "#@data/values-schema\n---\n#@overlay/match-child-defaults missing_ok=True\napplication:\n  extra: \"\"\n")
	write("prototypes/test-proto/vendir/vendir.yaml", "apiVersion: vendir.k14s.io/v1alpha1\nkind: Config\n")
	app.yttDataFiles = []string{schema, proto, env, again}

	sources, redundant, err := app.inspectOverrides()
	require.NoError(t, err)

	sourceOf := map[string]string{}
	for _, s := range sources {
		sourceOf[s.Path] = s.File
	}
	assert.Equal(t, proto, sourceOf["application.name"])
	assert.Equal(t, env, sourceOf["application.version"])
	assert.Equal(t, env, sourceOf["application.hosts"])
	assert.Equal(t, env, sourceOf["application.extra"])
	assert.Empty(t, sourceOf["helm.namespace"], "untouched values come from the defaults")

	assert.Equal(t, []InspectRedundantOverride{
		{Path: "application.version", File: proto, Value: "1.0"},
		{Path: "application.hosts", File: again, Value: []any{"a"}},
		{Path: "application.name", File: again, Value: "app"},
		{Path: "application.version", File: vendir, Value: "2.0"},
	}, redundant)
}

func Test_inspectOverridesKclMode(t *testing.T) {
	app := newTestApp(t)
	app.e.kclMode = true
	_, _, err := app.inspectOverrides()
	assert.ErrorContains(t, err, "not supported in KCL mode")
}
