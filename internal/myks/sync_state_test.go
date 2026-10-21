package myks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnsyncedReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		hasVendir  bool
		linksMap   string
		linkTarget bool
		want       string
	}{
		{name: "no vendir config", want: ""},
		{name: "never synced", hasVendir: true, want: "vendir sources were never synced"},
		{name: "cache entry removed", hasVendir: true, linksMap: "charts/nginx: abc\n", want: "vendored sources are missing: [charts/nginx]"},
		{name: "synced", hasVendir: true, linksMap: "charts/nginx: abc\n", linkTarget: true, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rootDir := t.TempDir()
			cfg := &Config{
				RootDir:                rootDir,
				ServiceDirName:         testServiceDir,
				AppsDir:                "_apps",
				PrototypesDir:          "prototypes",
				VendirStepDirName:      "vendir",
				VendorDirName:          "vendor",
				VendirLinksMapFileName: "vendir-links.yaml",
			}
			envDir := filepath.Join(rootDir, "envs", testEnvID)
			protoDir := filepath.Join(rootDir, "prototypes", "proto")
			require.NoError(t, os.MkdirAll(envDir, 0o700))
			require.NoError(t, os.MkdirAll(protoDir, 0o700))
			if tt.hasVendir {
				require.NoError(t, os.MkdirAll(filepath.Join(protoDir, "vendir"), 0o700))
			}

			app := &Application{
				Name:      testAppName,
				Prototype: protoDir,
				cfg:       cfg,
				e:         &Environment{ID: testEnvID, Dir: envDir, cfg: cfg},
			}
			if tt.linksMap != "" {
				require.NoError(t, writeFile(app.getLinksMapPath(), []byte(tt.linksMap)))
				link := app.expandVendorPath("charts/nginx")
				cacheData := filepath.Join(rootDir, "cache-data")
				require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o700))
				require.NoError(t, os.Symlink(cacheData, link))
				if tt.linkTarget {
					require.NoError(t, os.MkdirAll(cacheData, 0o700))
				}
			}

			got, err := app.unsyncedReason()
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
