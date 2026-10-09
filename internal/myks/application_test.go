package myks

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestApplication_renderDataYaml(t *testing.T) {
	type args struct {
		dataFiles []string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"happy path", args{[]string{"./assets/data-schema.ytt.yaml", "../../testData/ytt/data-file-schema.yaml", "../../testData/ytt/data-file-schema-2.yaml", "../../testData/ytt/data-file-values.yaml"}}, "application:\n  cache:\n    enabled: true\n  name: cert-manager\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := testApp.renderDataYaml(tt.args.dataFiles)
			if (err != nil) != tt.wantErr {
				t.Errorf("renderDataYaml() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !strings.Contains(string(got), tt.want) {
				t.Errorf("renderDataYaml() does not include expected string. got = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestNewApplication_skipAppData(t *testing.T) {
	g := NewWithDefaults()
	g.RootDir = t.TempDir()
	if err := os.MkdirAll(filepath.Join(g.RootDir, g.PrototypesDir, "proto"), 0o750); err != nil {
		t.Fatal(err)
	}
	env := &Environment{g: g, cfg: &g.Config}

	// Without data files, rendering application data values fails; the roster init must not attempt it.
	if _, err := NewApplication(env, "app", "proto"); err == nil {
		t.Fatal("NewApplication() expected an error from rendering data values")
	}
	g.skipAppData = true
	if _, err := NewApplication(env, "app", "proto"); err != nil {
		t.Fatalf("NewApplication() with skipAppData error = %v", err)
	}
}

func TestApplication_prototypeDir(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"happy path", testAppName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := testApp.prototypeDirName()
			if !reflect.DeepEqual(string(got), tt.want) {
				t.Errorf("prototypeDir() got = %v, want %v", got, tt.want)
			}
		})
	}
}
