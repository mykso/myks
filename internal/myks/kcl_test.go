package myks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

// TestMatchEnvFilter verifies env-dir matching against the CLI env/app selection.
func TestMatchEnvFilter(t *testing.T) {
	t.Parallel()
	sep := string(filepath.Separator)
	tests := []struct {
		// name of the test case
		name string
		// dir is the environment directory to match
		dir string
		// filter is the CLI env/app selection
		filter EnvAppMap
		// wantApps is the expected application selection (nil means all)
		wantApps []string
		// wantMatched is the expected match result
		wantMatched bool
	}{
		{"empty filter matches all", "envs" + sep + "dev", EnvAppMap{}, nil, true},
		{"exact match", "envs" + sep + "dev", EnvAppMap{"envs" + sep + "dev": {"app1"}}, []string{"app1"}, true},
		{"prefix match", "envs" + sep + "dev" + sep + "east", EnvAppMap{"envs" + sep + "dev": {"app1"}}, []string{"app1"}, true},
		{"no match", "envs" + sep + "prod", EnvAppMap{"envs" + sep + "dev": nil}, nil, false},
		{"nil apps mean all", "envs" + sep + "dev", EnvAppMap{"envs": nil}, nil, true},
		{"partial dir name is not a prefix match", "envs" + sep + "dev2", EnvAppMap{"envs" + sep + "dev": nil}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			apps, matched := matchEnvFilter(tt.dir, tt.filter)
			assert.Equal(t, tt.wantMatched, matched)
			assert.Equal(t, tt.wantApps, apps)
		})
	}
}

// TestKclEnvironmentDataValuesYaml verifies serialization of a tree entry into env data values.
func TestKclEnvironmentDataValuesYaml(t *testing.T) {
	t.Parallel()
	envData := kclEnvironmentData{
		ID:     "kcl-dev",
		ArgoCD: map[string]any{"enabled": false},
		Applications: map[string]map[string]any{
			"hello": {"proto": "hello-proto"},
			"world": {},
		},
	}
	yamlBytes, err := envData.dataValuesYaml()
	require.NoError(t, err)

	var parsed struct {
		ArgoCD      map[string]any `yaml:"argocd"`
		Environment struct {
			ID           string `yaml:"id"`
			Applications []struct {
				Name  string `yaml:"name"`
				Proto string `yaml:"proto"`
			} `yaml:"applications"`
		} `yaml:"environment"`
	}
	require.NoError(t, yaml.Unmarshal(yamlBytes, &parsed))
	assert.Equal(t, "kcl-dev", parsed.Environment.ID)
	assert.Equal(t, map[string]any{"enabled": false}, parsed.ArgoCD)
	require.Len(t, parsed.Environment.Applications, 2)
	assert.Equal(t, "hello", parsed.Environment.Applications[0].Name)
	assert.Equal(t, "hello-proto", parsed.Environment.Applications[0].Proto)
	// proto falls back to the application name when absent
	assert.Equal(t, "world", parsed.Environment.Applications[1].Proto)
}

// TestWriteKclAppDataFile verifies the generated data-values bridge file content.
func TestWriteKclAppDataFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "app-data.kcl-generated.ytt.yaml")
	appConfig := map[string]any{
		"proto":       "hello",
		"application": map[string]any{"greeting": "hi"},
	}
	require.NoError(t, writeKclAppDataFile(path, appConfig))

	content, err := os.ReadFile(path) // #nosec G304 -- test-controlled path
	require.NoError(t, err)
	assert.Contains(t, string(content), "#@data/values-schema")

	var values map[string]any
	require.NoError(t, yaml.Unmarshal(content, &values))
	assert.Equal(t, map[string]any{"greeting": "hi"}, values["application"])
	assert.NotContains(t, values, "proto", "engine-only key must not leak into data values")
	// the source map is left intact
	assert.Contains(t, appConfig, "proto")
}

// TestEvalKclTree verifies evaluation and validation of a minimal KCL module.
func TestEvalKclTree(t *testing.T) {
	t.Parallel()
	writeModule := func(t *testing.T, mainK string) string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "kcl.mod"), []byte("[package]\nname = \"config\"\n"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.k"), []byte(mainK), 0o600))
		return dir
	}

	t.Run("valid tree", func(t *testing.T) {
		t.Parallel()
		dir := writeModule(t, `
myksSchemaVersion = "0.1.0"
environments = {
    dev = {
        id = "kcl-dev"
        applications.hello.proto = "hello"
    }
}
`)
		tree, err := evalKclTree(dir)
		require.NoError(t, err)
		assert.Equal(t, "0.1.0", tree.MyksSchemaVersion)
		require.Contains(t, tree.Environments, "dev")
		assert.Equal(t, "kcl-dev", tree.Environments["dev"].ID)
		assert.Equal(t, "hello", tree.Environments["dev"].Applications["hello"]["proto"])
	})

	t.Run("missing schema version", func(t *testing.T) {
		t.Parallel()
		dir := writeModule(t, `environments = {}`)
		_, err := evalKclTree(dir)
		assert.ErrorContains(t, err, "myksSchemaVersion")
	})

	t.Run("missing environments", func(t *testing.T) {
		t.Parallel()
		dir := writeModule(t, `myksSchemaVersion = "0.1.0"`)
		_, err := evalKclTree(dir)
		assert.ErrorContains(t, err, "missing environments")
	})

	t.Run("empty environments are valid", func(t *testing.T) {
		t.Parallel()
		dir := writeModule(t, `
myksSchemaVersion = "0.1.0"
environments = {}
`)
		tree, err := evalKclTree(dir)
		require.NoError(t, err)
		assert.Empty(t, tree.Environments)
	})

	t.Run("duplicate environment ids", func(t *testing.T) {
		t.Parallel()
		dir := writeModule(t, `
myksSchemaVersion = "0.1.0"
environments = {
    dev = {id = "same"}
    prod = {id = "same"}
}
`)
		_, err := evalKclTree(dir)
		assert.ErrorContains(t, err, `duplicate environment id "same"`)
	})
}
