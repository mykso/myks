package myks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironment_renderEnvDataLib(t *testing.T) {
	// Setup
	env := &Environment{}
	envDataYaml := []byte(`foo: bar
baz:
  qux: 123
`)
	expectedLib := `
#@ def _env_data():
foo: bar
baz:
  qux: 123

#@ end

#@ load("@ytt:struct", "struct")
#@ load("@ytt:yaml", "yaml")
#@ env_data = struct.encode(yaml.decode(yaml.encode(_env_data())))
`

	// Test
	result, err := env.renderEnvDataLib(envDataYaml)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, expectedLib, string(result))
}

func TestEnvironment_initEnvData_CreatesLibFile(t *testing.T) {
	// Setup temporary directory
	tmpDir := t.TempDir()

	// Define paths
	envName := "my-env"
	envDir := filepath.Join(tmpDir, "envs", envName)
	err := os.MkdirAll(envDir, 0o750)
	require.NoError(t, err)

	envDataFileName := "env-data.ytt.yaml"
	envDataFile := filepath.Join(envDir, envDataFileName)

	// Create environment data file
	envDataContent := []byte(`
#@data/values
---
environment:
  id: ` + envName + `
`)
	err = os.WriteFile(envDataFile, envDataContent, 0o600)
	require.NoError(t, err)

	// Initialize Globe
	g := NewWithDefaults()
	g.RootDir = tmpDir
	g.EnvironmentDataFileName = envDataFileName

	// Initialize Environment
	env, err := NewEnvironment(g, envDir, envDataFile)
	require.NoError(t, err)

	// Test initEnvData
	err = env.initEnvData()
	require.NoError(t, err)

	// Verify lib file exists
	assert.FileExists(t, env.renderedDataLibFilePath)

	// Verify content of lib file
	content, err := os.ReadFile(env.renderedDataLibFilePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "environment:")
	assert.Contains(t, string(content), "id: "+envName)
	assert.Contains(t, string(content), "#@ def _env_data():")
}

func TestEnvironment_saveRenderedEnvDataLib(t *testing.T) {
	// Setup temporary directory
	tmpDir := t.TempDir()
	renderedEnvDataFilePath := filepath.Join(tmpDir, "env-data.lib.yaml")

	env := &Environment{
		renderedDataLibFilePath: renderedEnvDataFilePath,
	}
	data := []byte("some data")

	// Test
	err := env.saveRenderedEnvDataLib(data)

	// Verify
	require.NoError(t, err)
	assert.FileExists(t, renderedEnvDataFilePath)
}
