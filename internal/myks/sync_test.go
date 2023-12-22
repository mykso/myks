package myks

import (
	"os"
	"path/filepath"
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
				"charts/nginx",
			},
			wantErr: true,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const keeperFile = "keeper"
			app.Name = tt.name
			vendorDir := app.expandPath(app.e.g.VendorDirName)
			vendirConfigFile := app.expandServicePath(app.e.g.VendirConfigFileName)

			if err := os.MkdirAll(vendorDir, 0755); err != nil {
				t.Errorf("creating directory %s error = %v", vendorDir, err)
			}

			if err := os.MkdirAll(filepath.Dir(vendirConfigFile), 0755); err != nil {
				t.Errorf("creating directory %s error = %v", filepath.Dir(vendirConfigFile), err)
			}
			if err := os.WriteFile(vendirConfigFile, []byte(tt.vendirYaml), 0644); err != nil {
				t.Errorf("writing file %s error = %v", vendirConfigFile, err)
			}

			for _, dir := range tt.createDirs {
				if err := os.MkdirAll(filepath.Join(vendorDir, dir), 0755); err != nil {
					t.Errorf("creating directory %s error = %v", dir, err)
				}
				if err := os.WriteFile(filepath.Join(vendorDir, dir, keeperFile), []byte(""), 0644); err != nil {
					t.Errorf("writing file %s error = %v", vendirConfigFile, err)
				}
			}

			err := app.cleanupVendorDir(vendorDir, vendirConfigFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("cleanupVendorDir() error = %v, wantErr %v", err, tt.wantErr)
			}

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
