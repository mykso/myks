package myks

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/creasty/defaults"
)

func Test_cleanupVendorDir(t *testing.T) {
	tmpDir := t.TempDir()
	defer chdir(t, tmpDir)()

	g := &Globe{}
	if err := defaults.Set(g); err != nil {
		t.Fatal(err)
	}

	app := Application{
		e: &Environment{
			g:   g,
			Dir: "test-env",
		},
	}

	tests := []struct {
		name       string
		vendirYaml string
		createDirs []string
		wantDirs   []string
		wantErr    bool
	}{
		{
			name:       "empty",
			vendirYaml: "",
			createDirs: []string{},
			wantDirs:   []string{},
			wantErr:    true,
		},
		{
			name:       "no-vendir",
			vendirYaml: "",
			createDirs: []string{"something"},
			wantDirs:   []string{"something"},
			wantErr:    true,
		},
		{
			name: "exact-match",
			vendirYaml: "directories:\n" +
				"  - path: charts/httpbingo\n",
			createDirs: []string{"charts/httpbingo"},
			wantDirs:   []string{"charts/httpbingo"},
		},
		{
			name: "created-more",
			vendirYaml: "directories:\n" +
				"  - path: charts/httpbingo\n",
			createDirs: []string{
				"charts/httpbingo",
				"charts/drop1",
				"charts/drop2",
			},
			wantDirs: []string{"charts/httpbingo"},
		},
		{
			name: "created-less",
			vendirYaml: "directories:\n" +
				"  - path: charts/httpbingo\n" +
				"  - path: charts/nginx\n",
			createDirs: []string{
				"charts/httpbingo",
			},
			wantDirs: []string{
				"charts/httpbingo",
			},
		},
		{
			name: "multiple-dirs",
			vendirYaml: "directories:\n" +
				"  - path: charts/httpbingo\n" +
				"  - path: ytt/httpbingo\n" +
				"  - path: charts/nginx\n",
			createDirs: []string{
				"charts/httpbingo",
				"ytt/httpbingo",
				"charts/nginx",
			},
			wantDirs: []string{
				"charts/httpbingo",
				"ytt/httpbingo",
				"charts/nginx",
			},
		},
		{
			name: "similar-names",
			vendirYaml: "directories:\n" +
				"  - path: charts/httpbingo2\n",
			createDirs: []string{
				"charts/httpbingo",
				"charts/httpbingo2",
				"charts/httpbingo3",
				"charts/httpbingo22",
			},
			wantDirs: []string{"charts/httpbingo2"},
		},
		{
			name: "weird-paths",
			vendirYaml: "directories:\n" +
				"  - path: charts/httpbingo\n" +
				"  - path: charts/httpbingo1/\n" +
				"  - path: charts/httpbingo2//\n" +
				"  - path: charts//httpbingo22//\n" +
				"  - path: charts/../httpbingo69\n",
			createDirs: []string{
				"charts/httpbingo",
				"charts/httpbingo1",
				"charts/httpbingo2",
				"charts/httpbingo22",
				"httpbingo69",
				"charts/trash",
				"charts/trash1",
			},
			wantDirs: []string{
				"charts/httpbingo",
				"charts/httpbingo1",
				"charts/httpbingo2",
				"charts/httpbingo22",
				"httpbingo69",
			},
		},
	}

	vendorPathRegex := regexp.MustCompile(`path: (\S*)`)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const keeperFile = "keeper"
			app.Name = tt.name
			vendorDir := app.expandPath(app.e.g.VendorDirName)
			vendirConfigFile := app.expandServicePath(app.e.g.VendirConfigFileName)

			// Create vendor directory
			if err := os.MkdirAll(vendorDir, 0o755); err != nil {
				t.Errorf("creating directory %s error = %v", vendorDir, err)
			}

			// Prepend vendor directory to paths in vendir config
			tt.vendirYaml = vendorPathRegex.ReplaceAllString(tt.vendirYaml, "path: "+vendorDir+"/$1")

			// Create vendir config file
			if err := os.MkdirAll(filepath.Dir(vendirConfigFile), 0o755); err != nil {
				t.Errorf("creating directory %s error = %v", filepath.Dir(vendirConfigFile), err)
			}
			if err := os.WriteFile(vendirConfigFile, []byte(tt.vendirYaml), 0o644); err != nil {
				t.Errorf("writing file %s error = %v", vendirConfigFile, err)
			}

			// Create directories in vendor directory
			for _, dir := range tt.createDirs {
				if err := os.MkdirAll(filepath.Join(vendorDir, dir), 0o755); err != nil {
					t.Errorf("creating directory %s error = %v", dir, err)
				}
				if err := os.WriteFile(filepath.Join(vendorDir, dir, keeperFile), []byte(""), 0o644); err != nil {
					t.Errorf("writing file %s error = %v", vendirConfigFile, err)
				}
			}

			// Run the tested function, it should cleanup directories in vendor directory that are not in vendir config
			vendir := VendirSyncer{}
			err := vendir.cleanupVendorDir(&app, vendorDir, vendirConfigFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("cleanupVendorDir() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Then remove directories expected to left in vendor directory.
			// If there are any left, it means the cleanup failed.

			for _, dir := range tt.wantDirs {
				if _, err := os.Stat(filepath.Join(vendorDir, dir)); err != nil {
					t.Errorf("checking directory %s error = %v", dir, err)
				}
				os.RemoveAll(filepath.Join(vendorDir, dir))
			}

			leftFiles := []string{}
			if err := filepath.WalkDir(vendorDir, func(path string, info os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && info.Name() == keeperFile {
					leftFiles = append(leftFiles, path)
				}
				return nil
			}); err != nil {
				t.Errorf("collecting left files error = %v", err)
			}

			if len(leftFiles) > 0 {
				t.Errorf("vendor directory %s is not clean (%v files)", vendorDir, leftFiles)
			}
		})
	}
}
