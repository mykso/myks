package myks

import (
	"reflect"
	"testing"
)

func Test_unmarshalYaml(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]any
		wantErr bool
	}{
		{"happy path", args{"../../testData/util/yaml/simple.yaml"}, map[string]any{"key1": "A", "key2": "B", "arr": []any{"arr1", "arr2"}}, false},
		{"file not exist", args{"non-existing.yaml"}, map[string]any{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unmarshalYamlToMap(tt.args.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("unmarshalYamlToMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unmarshalYamlToMap() got = %v, wantArgs %v", got, tt.want)
			}
		})
	}
}
