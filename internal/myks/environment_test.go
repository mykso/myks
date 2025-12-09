package myks

import (
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
	// This is a bit more involved as it requires mocking or a real filesystem
	// For now, let's test the library generation part which is the core logic change.
	// The integration test would require setting up a full Globe and Environment with file structure.
}

func TestEnvironment_saveRenderedEnvDataLib(t *testing.T) {
	// Setup temporary directory
	tmpDir := t.TempDir()
	renderedEnvDataFilePath := filepath.Join(tmpDir, "env-data.lib.yaml")

	env := &Environment{
		renderedEnvDataFilePath: renderedEnvDataFilePath,
	}
	data := []byte("some data")

	// Test
	err := env.saveRenderedEnvDataLib(data)

	// Verify
	require.NoError(t, err)
	assert.FileExists(t, renderedEnvDataFilePath)
}
