package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindPluginsResolvesRelativeSourcesAgainstRootDir(t *testing.T) {
	rootDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(rootDir, "plugins"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "plugins", "hello"), []byte("#!/bin/sh\n"), 0o700))
	require.NoError(t, os.Mkdir(filepath.Join(rootDir, "sub"), 0o750))
	t.Chdir(filepath.Join(rootDir, "sub"))
	t.Setenv("PATH", "")

	prevRootDir, prevSources := viper.Get("root-dir"), viper.Get("plugin-sources")
	t.Cleanup(func() {
		viper.Set("root-dir", prevRootDir)
		viper.Set("plugin-sources", prevSources)
	})
	viper.Set("root-dir", rootDir)
	viper.Set("plugin-sources", []string{"./plugins", ""})

	var names []string
	for _, p := range findPlugins() {
		names = append(names, p.Name())
	}
	assert.Equal(t, []string{"hello"}, names)
}
