package myks

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestApplication_readSyncFile(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     []Directory
		wantErr  bool
	}{
		{
			"happy path",
			"../../testData/sync/sync-file.yaml",
			[]Directory{{Path: "path", ContentHash: "hash"}, {Path: "path2", ContentHash: "hash2"}},
			false,
		},
		{
			"no sync file",
			"no-existing.yaml",
			[]Directory{},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// write sync file
			var dirs []Directory
			var err error
			if dirs, err = readSyncFile(tt.filePath); (err != nil) != tt.wantErr {
				t.Errorf("writeSyncFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(dirs, tt.want) {
				t.Errorf("got = %v, wantArgs %v", dirs, tt.want)
			}
		})
	}
}

func Test_checkVersionMatch(t *testing.T) {
	type args struct {
		path        string
		contentHash string
		syncDirs    []Directory
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"happy path", args{"path1", "hash1", []Directory{{ContentHash: "hash1", Path: "path1"}}}, true},
		{"sad path", args{"path1", "hash1", []Directory{{ContentHash: "no-match", Path: "path1"}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkVersionMatch(tt.args.path, tt.args.contentHash, tt.args.syncDirs); got != tt.want {
				t.Errorf("checkVersionMatch() = %v, wantArgs %v", got, tt.want)
			}
		})
	}
}

func Test_findDirectories(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []Directory
		wantErr bool
	}{
		{
			"happy path",
			"../../testData/sync/vendir-simple.yaml",
			[]Directory{{ContentHash: "6fc0b0703de83385531372f85eae1763ae6af7068ec0b420abd5562adec2a01f", Path: "vendor/charts/loki-stack", Secret: "loki-secret"}},
			false,
		},
		{
			"yaml order irrelevant for hash",
			"../../testData/sync/vendir-simple-different-order.yaml",
			[]Directory{{ContentHash: "5589fa11a8117eefbec30e4190b9649dd282bd747b4acbd6e47201700990870b", Path: "vendor/charts/loki-stack"}},
			false,
		},
		{
			"multiple directories",
			"../../testData/sync/vendir-multiple-directories.yaml",
			[]Directory{
				{ContentHash: "84bc14f63b966dcec26278cc66976cdba19a8757f5b06f2be463e8033c8ade9c", Path: "vendor/charts/ingress-nginx"},
				{ContentHash: "4f95153c2130e5967fc97f0977877012b3f1579e6fcd9e66184302252ca83c70", Path: "vendor/ytt/grafana-dashboards"},
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
			nil,
			true,
		},
		{
			"with sub path",
			"../../testData/sync/vendir-with-subpath.yaml",
			[]Directory{
				{ContentHash: "5fa245cedee795a9a01fc62f3c56ac809dc8b304f6656897d060b68b8a5f32ef", Path: "vendor/charts/loki-stack"},
			},
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
			got, err := findDirectories(yaml)
			if (err != nil) != tt.wantErr {
				t.Errorf("findDirectories() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findDirectories() got = %v, wantArgs %v", got, tt.want)
			}
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
		want    []Directory
		wantErr bool
	}{
		{"happy path", args{"../../testData/sync/lock-file.yaml"}, []Directory{{Path: "vendor/charts/loki-stack", ContentHash: "9ebaa03dc8dd419b94a124193f6b597037daa95e208febb0122ca8920667f42a"}, {Path: "vendor/charts/ingress-nginx", ContentHash: "1d535ff265861947e32c890cbcb76d93a9562771dbd7b3367e4d723c1c6d95db"}}, false},
		{"file not exist", args{"file-not-exist.yaml"}, []Directory{}, false},
		{"no lock file", args{"../../testData/sync/simple.yaml"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readLockFile(tt.args.vendirLockFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("readLockFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readLockFile() got = %v, wantArgs %v", got, tt.want)
			}
		})
	}
}

func Test_checkPathMatch(t *testing.T) {
	type args struct {
		path     string
		syncDirs []Directory
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"happy path", args{"path1", []Directory{{Path: "path1"}}}, true},
		{"sad path", args{"non-existing", []Directory{{Path: "path1"}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkPathMatch(tt.args.path, tt.args.syncDirs); got != tt.want {
				t.Errorf("checkPathMatch() = %v, wantArgs %v", got, tt.want)
			}
		})
	}
}

func Test_checkLockFileMatch(t *testing.T) {
	type args struct {
		vendirDirs   []Directory
		lockFileDirs []Directory
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"happy path", args{[]Directory{{Path: "path1"}}, []Directory{{Path: "path1"}}}, true},
		{"sad path", args{[]Directory{{Path: "path2"}}, []Directory{{Path: "path1"}}}, false},
		{"wrong sort order", args{[]Directory{{Path: "path1"}, {Path: "path2"}}, []Directory{{Path: "path2"}, {Path: "path1"}}}, true},
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
		want    []Directory
		wantErr bool
	}{
		{"happy path", args{"../../testData/sync/vendir-simple.yaml"}, []Directory{{Path: "vendor/charts/loki-stack", ContentHash: "6fc0b0703de83385531372f85eae1763ae6af7068ec0b420abd5562adec2a01f", Secret: "loki-secret"}}, false},
		{"oci image", args{"../../testData/sync/vendir-oci.yaml"}, []Directory{{Path: "vendor/ytt/grafana", ContentHash: "11b1e2b989d81bb8daffc10f7be4d059bc0eec684913732fbfdadabbe79c7fb2", Secret: "grafana-secret"}}, false},
		{"file not exist", args{"file-not-exist.yaml"}, nil, true},
		{"no vendir file", args{"../../testData/sync/simple.yaml"}, nil, true}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readVendirConfig(tt.args.vendirConfigFilePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("readVendirConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readVendirConfig() got = %v, wantArgs %v", got, tt.want)
			}
		})
	}
}

func Test_getEnvCreds(t *testing.T) {
	type args struct {
		secretName       string
		envUsernameKey   string
		envUsernameValue string
		envPasswordKey   string
		envPasswordValue string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{"happy path", args{"loki-secret", secretEnvPrefix + "LOKI-SECRET_USERNAME", "username", secretEnvPrefix + "LOKI-SECRET_PASSWORD", "password"}, "username", "password", false},
		{"sad path", args{"loki-secret", secretEnvPrefix + "LOKI-SECRET_USERNAME", "", secretEnvPrefix + "LOKI-SECRET_PASSWORD", ""}, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnvSafely(tt.args.envUsernameKey, tt.args.envUsernameValue, t)
			setEnvSafely(tt.args.envPasswordKey, tt.args.envPasswordValue, t)
			got, got1, err := getEnvCreds(tt.args.secretName)
			if (err != nil) != tt.wantErr {
				t.Errorf("getEnvCreds() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getEnvCreds() got = %v, wantArgs %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("getEnvCreds() got1 = %v, wantArgs %v", got1, tt.want1)
			}
		})
	}
}

func Test_writeSyncFile(t *testing.T) {
	type args struct {
		syncFilePath string
		directories  []Directory
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			"happy path",
			args{filepath.Join(os.TempDir(), "testfile"), []Directory{{Path: "path", ContentHash: "hash"}, {Path: "path2", ContentHash: "hash2"}}},
			"- path: path\n  contentHash: hash\n- path: path2\n  contentHash: hash2\n",
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
				t.Errorf("writeFile() error = %v", err)
			}
			if string(file) != tt.want {
				t.Errorf("writeSecretFile() got = %v, wantArgs %v", file, tt.want)
			}
		})
	}
}

func Test_handleVendirSecret(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping in pipeline since ytt is not installed")
	}
	type args struct {
		dir              Directory
		tempPath         string
		tempRelativePath string
		vendirArgs       []string
		envUsernameKey   string
		envUsernameValue string
		envPasswordKey   string
		envPasswordValue string
	}
	tests := []struct {
		name     string
		args     args
		wantArgs []string
		wantPath string
		wantErr  bool
	}{
		{"no secret", args{Directory{}, "", "", []string{}, "", "", "", ""}, []string{}, "", false},
		{"no credentials", args{Directory{Secret: "test-secret"}, "", "", []string{}, "", "", "", ""}, []string{}, "", true},
		{
			"secret",
			args{Directory{Secret: "test-secret"}, os.TempDir(), os.TempDir(), []string{}, "VENDIR_SECRET_TEST-SECRET_USERNAME", "test", "VENDIR_SECRET_TEST-SECRET_PASSWORD", "test"},
			[]string{"--file=" + filepath.Join(os.TempDir(), "test-secret.yaml")},
			filepath.Join(os.TempDir(), "test-secret.yaml"),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnvSafely(tt.args.envUsernameKey, tt.args.envUsernameValue, t)
			setEnvSafely(tt.args.envPasswordKey, tt.args.envPasswordValue, t)
			got, secretPath, err := handleVendirSecret(tt.args.dir, tt.args.tempPath, tt.args.tempRelativePath, tt.args.vendirArgs)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleVendirSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if secretPath != tt.wantPath {
				t.Errorf("handleVendirSecret() secretPath = %v, wantPath %v", secretPath, tt.wantPath)
			}
			if !reflect.DeepEqual(got, tt.wantArgs) {
				t.Errorf("handleVendirSecret() got = %v, wantArgs %v", got, tt.wantArgs)
			}
		})
	}
}

func setEnvSafely(key string, value string, t *testing.T) {
	if key == "" {
		return
	}
	err := os.Setenv(key, value)
	if err != nil {
		t.Errorf("failed to set env variable %s", key)
	}
}
