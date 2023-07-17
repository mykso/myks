package myks

import (
	"reflect"
	"testing"
)

func Test_hash(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want string
	}{
		{"happy path", "some-string", "a3635c09bda7293ae1f144a240f155cf151451f2420d11ac385d13cce4eb5fa2"},
	}
	for _, tt := range tests {
		t.Run(tt.a, func(t *testing.T) {
			if got := hash(tt.b); got != tt.want {
				t.Errorf("hash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sortYaml(t *testing.T) {

	tests := []struct {
		name    string
		args    map[string]interface{}
		want    string
		wantErr bool
	}{
		{
			"happy path",
			map[string]interface{}{"key1": "A", "key2": "B"},
			"map[key1:A key2:B]",
			false,
		},
		{
			"fix sorting",
			map[string]interface{}{"key2": "B", "key1": "A"},
			"map[key1:A key2:B]",
			false,
		},
		{
			"empty input",
			nil,
			"",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sortYaml(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("sortYaml() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("sortYaml() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_unmarshalYaml(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{"happy path", args{"../../testData/sync/simple.yaml"}, map[string]interface{}{"key1": "A", "key2": "B", "arr": []interface{}{"arr1", "arr2"}}, false},
		{"file not exist", args{"non-existing.yaml"}, map[string]interface{}{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unmarshalYamlToMap(tt.args.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("unmarshalYamlToMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unmarshalYamlToMap() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_renderDataYaml(t *testing.T) {
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
			got, err := renderDataYaml(tt.args.dataFiles)
			if (err != nil) != tt.wantErr {
				t.Errorf("renderDataYaml() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(string(got), tt.want) {
				t.Errorf("renderDataYaml() got = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func Test_createDirectory(t *testing.T) {
	type args struct {
		dir string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"happy path", args{"/tmp/test-dir"}, false},
		{"sad path", args{"/non-existing/test-dir"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := createDirectory(tt.args.dir); (err != nil) != tt.wantErr {
				t.Errorf("createDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
