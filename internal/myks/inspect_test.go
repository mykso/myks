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

// writeAppFile writes content to rel under the root of app and returns the full path.
func writeAppFile(t *testing.T, app *Application, rel, content string) string {
	t.Helper()
	path := filepath.Join(app.cfg.RootDir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
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
	schema := writeAppFile(t, app, "prototypes/test-proto/app-data.schema.yaml", "#@data/values-schema\n---\n#@overlay/match-child-defaults missing_ok=True\napplication:\n  #@schema/validation min_len=1\n  name: \"\"\n  version: \"1.0\"\n  hosts: [\"\"]\n  port: 80\n  replicas: 1\n")
	proto := writeAppFile(t, app, "prototypes/test-proto/app-data.yaml", "#@data/values\n---\napplication:\n  name: #@ \"app\"\n  version: \"1.0\"\nmyks:\n  gitRepoBranch: \"\"\n")
	env := writeAppFile(t, app, "envs/test-env/_apps/test-app/app-data.yaml", "#@data/values\n---\napplication:\n  name: app\n  version: \"2.0\"\n  hosts: [a]\n  extra: x\n")
	// The second document sets name again: one finding per file and key.
	again := writeAppFile(t, app, "envs/test-env/_apps/test-app/app-data.again.yaml", "#@data/values\n---\napplication:\n  #@overlay/replace\n  hosts: [a]\n  name: app\n#@data/values\n---\napplication:\n  name: app\n")
	// A schema applies before every data value, even one in an earlier file.
	lateSchema := writeAppFile(t, app, "envs/test-env/_apps/test-app/app-data.schema.yaml", "#@data/values-schema\n---\n#@overlay/match-child-defaults missing_ok=True\napplication:\n  extra: \"\"\n  replicas: 2\n")
	vendir := writeAppFile(t, app, "prototypes/test-proto/vendir/values.yaml", "#@data/values\n---\napplication:\n  version: \"2.0\"\n  chart: \"1.2\"\n  repo: r\n")
	vendirSchema := writeAppFile(t, app, "prototypes/test-proto/vendir/schema.yaml", "#@data/values-schema\n---\n#@overlay/match-child-defaults missing_ok=True\napplication:\n  chart: \"\"\n  repo: r\n")
	writeAppFile(t, app, "prototypes/test-proto/vendir/vendir.yaml", "apiVersion: vendir.k14s.io/v1alpha1\nkind: Config\n")
	// Data values of a private library never reach the root.
	writeAppFile(t, app, "prototypes/test-proto/vendir/_ytt_lib/lib/values.yaml", "#@data/values\n---\napplication:\n  version: \"2.0\"\n")
	app.yttDataFiles = []string{schema, proto, env, again, lateSchema}

	sources, redundant, err := app.inspectOverrides(true)
	require.NoError(t, err)

	sourceOf := map[string]string{}
	for _, s := range sources {
		sourceOf[s.Path] = s.File
	}
	assert.Equal(t, proto, sourceOf["application.name"])
	assert.Equal(t, env, sourceOf["application.version"])
	assert.Equal(t, env, sourceOf["application.hosts"])
	assert.Equal(t, env, sourceOf["application.extra"])
	assert.Equal(t, schema, sourceOf["application.port"], "a default is traced to its schema")
	assert.Equal(t, lateSchema, sourceOf["application.replicas"], "a later schema overrides an earlier default")
	assert.Empty(t, sourceOf["helm.namespace"], "untouched values come from the built-in defaults")
	assert.NotContains(t, sourceOf, "application.chart", "vendir data values never reach the render steps")

	assert.Equal(t, []InspectRedundantOverride{
		{Path: "application.version", File: proto, Value: "1.0", InheritedFrom: schema},
		{Path: "application.name", File: env, Value: "app", InheritedFrom: proto},
		{Path: "application.hosts", File: again, Value: []any{"a"}, InheritedFrom: env},
		// The redundant env file in between is skipped: proto holds the value.
		{Path: "application.name", File: again, Value: "app", InheritedFrom: proto},
		{Path: "application.repo", File: vendir, Value: "r", InheritedFrom: vendirSchema},
		{Path: "application.version", File: vendir, Value: "2.0", InheritedFrom: env},
	}, redundant)
}

func Test_inspectValuesOverrides(t *testing.T) {
	app := newTestApp(t)
	app.e.extraYttPaths = []string{"./assets/data-schema.ytt.yaml"}
	app.yttDataFiles = []string{writeAppFile(t, app, "prototypes/test-proto/app-data.yaml", "#@data/values-schema\n---\n#@overlay/match-child-defaults missing_ok=True\napplication:\n  name: app\n")}
	protoGlobal := writeAppFile(t, app, "prototypes/test-proto/helm/_global.yaml", "#@ load(\"@ytt:data\", \"data\")\n---\nchart: #@ data.values.myks.context.helm.chart\nreplicas: 1\nimage:\n  tag: v1\nmyks: x\n")
	protoWeb := writeAppFile(t, app, "prototypes/test-proto/helm/web.yaml", "#@ load(\"@ytt:data\", \"data\")\n---\nname: #@ data.values.application.name\nimage:\n  tag: v2\n")
	appGlobal := writeAppFile(t, app, "envs/test-env/_apps/test-app/helm/_global.yaml", "replicas: 1\nmyks: x\n")
	appWeb := writeAppFile(t, app, "envs/test-env/_apps/test-app/helm/web.yaml", "image:\n  tag: v2\nport: 8080\n")
	// Render never sees values files of a chart that is not vendored.
	writeAppFile(t, app, "envs/test-env/_apps/test-app/helm/gone.yaml", "replicas: 2\n")
	// A vendored chart without values files of its own still gets the global ones.
	for _, chart := range []string{"db", "web"} {
		require.NoError(t, os.MkdirAll(filepath.Join(app.expandVendorPath(app.cfg.HelmChartsDirName), chart), 0o700))
	}

	sources, redundant, err := app.inspectValuesOverrides(app.cfg.HelmStepDirName, app.cfg.HelmChartsDirName)
	require.NoError(t, err)

	assert.Equal(t, map[string][]InspectValueSource{
		"db": {
			{Path: "chart", File: protoGlobal},
			{Path: "image.tag", File: protoGlobal},
			{Path: "myks", File: protoGlobal},
			{Path: "replicas", File: protoGlobal},
		},
		"web": {
			{Path: "chart", File: protoGlobal},
			{Path: "image.tag", File: protoWeb},
			{Path: "myks", File: protoGlobal},
			{Path: "name", File: protoWeb},
			{Path: "port", File: appWeb},
			{Path: "replicas", File: protoGlobal},
		},
	}, sources)
	assert.Equal(t, map[string][]InspectRedundantOverride{
		// Unlike in data values, a myks key in a values file is no runtime data.
		"db": {
			{Path: "myks", File: appGlobal, Value: "x", InheritedFrom: protoGlobal},
			{Path: "replicas", File: appGlobal, Value: 1, InheritedFrom: protoGlobal},
		},
		"web": {
			{Path: "myks", File: appGlobal, Value: "x", InheritedFrom: protoGlobal},
			{Path: "replicas", File: appGlobal, Value: 1, InheritedFrom: protoGlobal},
			{Path: "image.tag", File: appWeb, Value: "v2", InheritedFrom: protoWeb},
		},
	}, redundant)
}

func Test_inspectInstanceOverridesKclMode(t *testing.T) {
	app := newTestApp(t)
	app.e.kclMode = true
	app.e.extraYttPaths = []string{"./assets/data-schema.ytt.yaml"}
	// Generated data values hold no override to trace; ytt-pkg values files are still checked.
	// Nothing is vendored yet, so the values file names give the packages.
	app.yttDataFiles = []string{writeAppFile(t, app, ".myks/envs/test-env/_apps/test-app/app-data.yaml", "#@data/values-schema\n---\n#@overlay/match-child-defaults missing_ok=True\napplication:\n  version: \"1.0\"\n")}
	protoPkg := writeAppFile(t, app, "prototypes/test-proto/ytt-pkg/pkg.yaml", "a: 1\n")
	appPkg := writeAppFile(t, app, "envs/test-env/_apps/test-app/ytt-pkg/pkg.yaml", "a: 1\n")

	var instance InspectAppInstance
	require.NoError(t, app.inspectInstanceOverrides(&instance, InspectAppsOptions{Provenance: true, Redundant: true}))
	assert.Empty(t, instance.Provenance)
	assert.Empty(t, instance.Redundant)
	assert.Equal(t, map[string][]InspectRedundantOverride{"pkg": {{Path: "a", File: appPkg, Value: 1, InheritedFrom: protoPkg}}}, instance.YttPkgRedundant)
}
