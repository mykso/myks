package myks

import (
	"io"
	"os"
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
				t.Errorf("hash() = %v, wantArgs %v", got, tt.want)
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
				t.Errorf("sortYaml() got = %v, wantArgs %v", got, tt.want)
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
				t.Errorf("unmarshalYamlToMap() got = %v, wantArgs %v", got, tt.want)
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

func Test_writeFile(t *testing.T) {
	type args struct {
		path    string
		content []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"happy path", args{"/tmp/test-file", []byte("test")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeFile(tt.args.path, tt.args.content); (err != nil) != tt.wantErr {
				t.Errorf("writeFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			file, err := os.ReadFile(tt.args.path)
			if err != nil {
				t.Errorf("writeFile() error = %v", err)
			}
			if string(file) != string(tt.args.content) {
				t.Errorf("writeFile() got = %v, wantArgs %v", string(file), string(tt.args.content))
			}
		})
	}
}

func Test_appendIfNotExists(t *testing.T) {
	type args struct {
		slice   []string
		element string
	}
	tests := []struct {
		name      string
		args      args
		wantArgs  []string
		wantAdded bool
	}{
		{"add dup", args{[]string{"test"}, "test"}, []string{"test"}, false},
		{"add new element", args{[]string{"test"}, "test2"}, []string{"test", "test2"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, added := appendIfNotExists(tt.args.slice, tt.args.element)
			if !reflect.DeepEqual(got, tt.wantArgs) {
				t.Errorf("appendIfNotExists() = %v, wantArgs %v", got, tt.wantArgs)
			}
			if !added == tt.wantAdded {
				t.Errorf("appendIfNotExists() = %v, wantAdded %v", got, tt.wantArgs)
			}
		})
	}
}

func Test_reductSecrets(t *testing.T) {
	type args struct {
		args []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{"happy path", args{[]string{"password=verysecret", "secret=verysecret", "token=verysecret"}}, []string{"password=[REDACTED]", "secret=[REDACTED]", "token=[REDACTED]"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reductSecrets(tt.args.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reductSecrets() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getSubDirs(t *testing.T) {
	type args struct {
		resourceDir string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{"happy path", args{"../../testData/vendor/charts"}, []string{"../../testData/vendor/charts/test-chart"}},
		{"empty", args{""}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getSubDirs(tt.args.resourceDir); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getSubDirs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_runCmd(t *testing.T) {
	type args struct {
		name  string
		stdin io.Reader
		args  []string
		log   func(name string, args []string)
	}
	tests := []struct {
		name    string
		args    args
		want    CmdResult
		wantErr bool
	}{
		{"happy path", args{"echo", nil, []string{"test"}, nil}, CmdResult{"test\n", ""}, false},
		{"said path", args{"sure-to-fail", nil, []string{}, nil}, CmdResult{"", ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runCmd(tt.args.name, tt.args.stdin, tt.args.args, tt.args.log)
			if (err != nil) != tt.wantErr {
				t.Errorf("runCmd() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("runCmd() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_runYttWithFilesAndStdin(t *testing.T) {
	type args struct {
		paths []string
		stdin io.Reader
		log   func(name string, args []string)
		args  []string
	}
	tests := []struct {
		name    string
		args    args
		want    CmdResult
		wantErr bool
	}{
		{"happy path", args{[]string{"../../testData/ytt/simple.yaml"}, nil, nil, []string{}}, CmdResult{"key1: A\n", ""}, false},
		{"said path", args{[]string{"does-not-exist.yaml"}, nil, nil, []string{}}, CmdResult{"", ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runYttWithFilesAndStdin(tt.args.paths, tt.args.stdin, tt.args.log, tt.args.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("runYttWithFilesAndStdin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("runYttWithFilesAndStdin() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
