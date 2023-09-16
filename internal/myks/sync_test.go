package myks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplication_readSyncFile(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     vendirDirHashes
		wantErr  bool
	}{
		{
			"happy path",
			"../../testData/sync/sync-file.yaml",
			vendirDirHashes{"path": "hash", "path2": "hash2"},
			false,
		},
		{
			"no sync file",
			"no-existing.yaml",
			vendirDirHashes{},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// write sync file
			var dirs vendirDirHashes
			var err error
			if dirs, err = readSyncFile(tt.filePath); (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			assertEqual(t, dirs, tt.want)
		})
	}
}

func Test_checkVersionMatch(t *testing.T) {
	type args struct {
		path        string
		contentHash string
		syncDirs    vendirDirHashes
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"happy path", args{"path1", "hash1", vendirDirHashes{"path1": "hash1"}}, true},
		{"sad path", args{"path1", "hash1", vendirDirHashes{"path1": "no-match"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkVersionMatch(tt.args.path, tt.args.contentHash, tt.args.syncDirs); got != tt.want {
				t.Errorf("checkVersionMatch() = %v, wantArgs %v", got, tt.want)
			}
		})
	}
}

func Test_getVendirDirHashes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    vendirDirHashes
		wantErr bool
	}{
		{
			"happy path",
			"../../testData/sync/vendir-simple.yaml",
			vendirDirHashes{"vendor/charts/loki-stack": "da992fbae34fe2c310026bef76eb03cf103743010c98a8a1922303a384833fdd"},
			false,
		},
		{
			"yaml order irrelevant for hash",
			"../../testData/sync/vendir-simple-different-order.yaml",
			vendirDirHashes{"vendor/charts/loki-stack": "64eb3e3e2af99bc1d5fd155b2edc4ed3b4721430919602cd6d11c76d3ab17d24"},
			false,
		},
		{
			"multiple directories",
			"../../testData/sync/vendir-multiple-directories.yaml",
			vendirDirHashes{
				"vendor/charts/ingress-nginx":   "3b52aa63642d9d9ab4bb3007ce67f1f0431d1791c4d4c78a544971d67728320a",
				"vendor/ytt/grafana-dashboards": "c068fe6a6572bf9fc0aeb87f70acd494122931b44ea4297a1297fb2f735b2723",
			},
			false,
		},
		{
			"not a vendir file",
			"../../testData/sync/simple.yaml",
			nil,
			true,
		},
		{
			"multiple contents",
			"../../testData/sync/vendir-multiple-contents.yaml",
			vendirDirHashes{"vendor/charts/ingress-nginx": "e9b262400008526b84cf46d99f844f84bbbef2abedc38a077ef7c6ec015ef6dd"},
			false,
		},
		{
			"with sub path",
			"../../testData/sync/vendir-with-subpath.yaml",
			vendirDirHashes{"vendor/charts": "92f1735562d38c44a735022f1ada170b7d286ef2e51f9cba9a3c67c83c9ecae0"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml, err := unmarshalYamlToMap(tt.input)
			if err != nil {
				t.Errorf("unmarshalYamlToMap() error = %v", err)
				return
			}
			got, err := getVendirDirHashes(yaml)
			if (err != nil) != tt.wantErr {
				t.Errorf("getVendirDirHashes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assertEqual(t, got, tt.want)
		})
	}
}

func Test_readLockFile(t *testing.T) {
	type args struct {
		vendirLockFile string
	}
	tests := []struct {
		name    string
		args    args
		want    vendirDirHashes
		wantErr bool
	}{
		{"happy path", args{"../../testData/sync/lock-file.yaml"}, vendirDirHashes{
			"vendor/charts":               "98d8127a386c0e1520c758783642a42d7cee97b32a8f255974ea3d48bc237f5a",
			"vendor/charts/ingress-nginx": "3eec412b32018cdad77f2b084719a142f034bb9df19866f0cf847641a4c27a96",
		}, false},
		{"file not exist", args{"file-not-exist.yaml"}, vendirDirHashes{}, false},
		{"no lock file", args{"../../testData/sync/simple.yaml"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readLockFileDirHashes(tt.args.vendirLockFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assertEqual(t, got, tt.want)
		})
	}
}

func Test_checkLockFileMatch(t *testing.T) {
	type args struct {
		vendirDirs   vendirDirHashes
		lockFileDirs vendirDirHashes
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"happy path", args{vendirDirHashes{"path1": ""}, vendirDirHashes{"path1": ""}}, true},
		{"sad path", args{vendirDirHashes{"path2": ""}, vendirDirHashes{"path1": ""}}, false},
		{"wrong sort order", args{vendirDirHashes{"path1": "", "path2": ""}, vendirDirHashes{"path2": "", "path1": ""}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkLockFileMatch(tt.args.vendirDirs, tt.args.lockFileDirs); got != tt.want {
				t.Errorf("checkLockFileMatch() = %v, wantArgs %v", got, tt.want)
			}
		})
	}
}

func Test_readVendirConfig(t *testing.T) {
	type args struct {
		vendirConfigFilePath string
	}
	tests := []struct {
		name    string
		args    args
		want    vendirDirHashes
		wantErr bool
	}{
		{
			"happy path",
			args{"../../testData/sync/vendir-simple.yaml"},
			vendirDirHashes{"vendor/charts/loki-stack": "da992fbae34fe2c310026bef76eb03cf103743010c98a8a1922303a384833fdd"},
			false,
		},
		{
			"oci image",
			args{"../../testData/sync/vendir-oci.yaml"},
			vendirDirHashes{"vendor/ytt/grafana": "cd9f99d5020ad7d19b5fa27919112ace76a2d5c0d948e22207b6cba8d1374f22"},
			false,
		},
		{"file not exist", args{"file-not-exist.yaml"}, nil, true},
		{"no vendir file", args{"../../testData/sync/simple.yaml"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readVendirDirHashes(tt.args.vendirConfigFilePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assertEqual(t, got, tt.want)
		})
	}
}

func Test_writeSyncFile(t *testing.T) {
	type args struct {
		syncFilePath string
		directories  vendirDirHashes
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			"happy path",
			args{
				filepath.Join(os.TempDir(), "testfile"),
				vendirDirHashes{
					"path":  "hash",
					"path2": "hash2",
				},
			},
			"path: hash\npath2: hash2\n",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeSyncFile(tt.args.syncFilePath, tt.args.directories); (err != nil) != tt.wantErr {
				t.Errorf("writeSyncFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			file, err := os.ReadFile(tt.args.syncFilePath)
			if err != nil {
				t.Errorf("os.ReadFile() error = %v", err)
			}
			if string(file) != tt.want {
				t.Errorf("got = %v, want %v", file, tt.want)
			}
		})
	}
}
