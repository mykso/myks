package myks

import (
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

func TestApplication_prototypeDir(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"happy path", "test-app"},
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
