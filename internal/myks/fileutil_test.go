package myks

import (
	"os"
	"testing"
)

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

func Test_getSubDirs(t *testing.T) {
	baseDir := testDataDir + "/getSubDirs"
	tests := []struct {
		name    string
		dir     string
		want    []string
		wantErr bool
	}{
		{"one subdir", baseDir + "/one", []string{baseDir + "/one/foo"}, false},
		{"two subdirs", baseDir + "/two", []string{baseDir + "/two/.baz", baseDir + "/two/bar"}, false},
		{"no subdirs", baseDir + "/none", nil, false},
		{"empty dir name", "", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getSubDirs(tt.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("error: %v, wantErr: %v", err, tt.wantErr)
			} else {
				assertEqual(t, got, tt.want)
			}
		})
	}
}
