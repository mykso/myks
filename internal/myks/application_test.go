package myks

import (
	"reflect"
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
		{"happy path", args{[]string{"../../testData/ytt/data-file-schema.yaml", "../../testData/ytt/data-file-schema-2.yaml", "../../testData/ytt/data-file-values.yaml"}}, "application:\n  cache:\n    enabled: true\n  name: cert-manager\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := testApp.renderDataYaml(tt.args.dataFiles)
			if (err != nil) != tt.wantErr {
				t.Errorf("renderDataYaml() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(string(got), tt.want) {
				t.Errorf("renderDataYaml() got = %v, want %v", got, tt.want)
			}
		})
	}
}
