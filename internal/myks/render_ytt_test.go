package myks

import (
	"reflect"
	"testing"
)

func Test_collectYttFiles(t *testing.T) {
	type args struct {
		resourceDir string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{"happy path", args{"../../testData/vendor/ytt/test-resource"}, []string{"../../testData/vendor/ytt/test-resource/config", "../../testData/vendor/ytt/test-resource/manifests"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collectYttFiles(tt.args.resourceDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("collectYttFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectYttFiles() got = %v, want %v", got, tt.want)
			}
		})
	}
}
