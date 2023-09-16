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
			vendirDirHashes{"vendor/charts/loki-stack": "6fc0b0703de83385531372f85eae1763ae6af7068ec0b420abd5562adec2a01f"},
			false,
		},
		{
			"yaml order irrelevant for hash",
			"../../testData/sync/vendir-simple-different-order.yaml",
			vendirDirHashes{"vendor/charts/loki-stack": "5589fa11a8117eefbec30e4190b9649dd282bd747b4acbd6e47201700990870b"},
			false,
		},
		{
			"multiple directories",
			"../../testData/sync/vendir-multiple-directories.yaml",
			vendirDirHashes{
				"vendor/charts/ingress-nginx":   "84bc14f63b966dcec26278cc66976cdba19a8757f5b06f2be463e8033c8ade9c",
				"vendor/ytt/grafana-dashboards": "4f95153c2130e5967fc97f0977877012b3f1579e6fcd9e66184302252ca83c70",
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
			vendirDirHashes{
				"vendor/charts/ingress-nginx/chart":      "38e595e0991357e484d8bdaf92c53d9b4d97c62d063222882fbb00f1732a7523",
				"vendor/charts/ingress-nginx/dashboards": "8dd23a97c0b896d94789d96afe1c278f7287d1580b9a58f5a6087fa44e684b46",
			},
			false,
		},
		{
			"with sub path",
			"../../testData/sync/vendir-with-subpath.yaml",
			vendirDirHashes{"vendor/charts/loki-stack": "5fa245cedee795a9a01fc62f3c56ac809dc8b304f6656897d060b68b8a5f32ef"},
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
			"vendor/charts/loki-stack":    "9ebaa03dc8dd419b94a124193f6b597037daa95e208febb0122ca8920667f42a",
			"vendor/charts/ingress-nginx": "1d535ff265861947e32c890cbcb76d93a9562771dbd7b3367e4d723c1c6d95db",
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
			vendirDirHashes{"vendor/charts/loki-stack": "6fc0b0703de83385531372f85eae1763ae6af7068ec0b420abd5562adec2a01f"},
			false,
		},
		{
			"oci image",
			args{"../../testData/sync/vendir-oci.yaml"},
			vendirDirHashes{"vendor/ytt/grafana": "11b1e2b989d81bb8daffc10f7be4d059bc0eec684913732fbfdadabbe79c7fb2"},
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
