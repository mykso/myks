package myks

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func Test_AddBaseDirToEnvPath(t *testing.T) {
	g := NewWithDefaults()
	tests := []struct {
		in  string
		out string
	}{
		// Here we use "envs" as the base directory.
		// Change it if the default value is changed at Glabe.EnvironmentBaseDir.
		{"foo", "envs/foo"},
		{"foo/bar", "envs/foo/bar"},
		{"envsy", "envs/envsy"},
		{"envs", "envs"},
		{"envs/", "envs/"},
		{"envs/foo", "envs/foo"},
	}
	for _, tt := range tests {
		out := g.AddBaseDirToEnvPath(tt.in)
		if out != tt.out {
			t.Errorf("addBaseDirToEnvPath(%s) = %s; want %s", tt.in, out, tt.out)
		}
	}
}

func Test_collectEnvironments(t *testing.T) {
	defer chdir(t, "../../testData/collect-environments")()

	g := NewWithDefaults()
	g.environments = make(map[string]*Environment)
	tests := []struct {
		name string
		in   EnvAppMap
		out  EnvAppMap
	}{
		{
			"empty map",
			EnvAppMap{},
			EnvAppMap{
				"envs/bar/prod":  []string{},
				"envs/bar/stage": []string{},
				"envs/foo/prod":  []string{},
				"envs/foo/stage": []string{},
			},
		},
		{
			"one env",
			EnvAppMap{
				"envs/bar/prod": []string{"app1", "app2"},
			},
			EnvAppMap{
				"envs/bar/prod": []string{"app1", "app2"},
			},
		},
		{
			"multiple envs",
			EnvAppMap{
				"envs/bar/prod":  []string{"app1", "app2"},
				"envs/bar/stage": []string{"app1", "app3"},
				"envs/foo/prod":  []string{"app1"},
			},
			EnvAppMap{
				"envs/bar/prod":  []string{"app1", "app2"},
				"envs/bar/stage": []string{"app1", "app3"},
				"envs/foo/prod":  []string{"app1"},
			},
		},
		{
			"nested envs",
			EnvAppMap{
				"envs":           []string{"app1", "envsApp"},
				"envs/bar":       []string{"app2", "barApp"},
				"envs/bar/stage": []string{"app3", "stageApp"},
			},
			EnvAppMap{
				"envs/bar/prod":  []string{"app1", "envsApp", "app2", "barApp"},
				"envs/bar/stage": []string{"app1", "envsApp", "app2", "barApp", "app3", "stageApp"},
				"envs/foo/prod":  []string{"app1", "envsApp"},
				"envs/foo/stage": []string{"app1", "envsApp"},
			},
		},
		{
			"deduplication",
			EnvAppMap{
				"envs/bar": []string{"app1", "app1", "app2"},
			},
			EnvAppMap{
				"envs/bar/prod":  []string{"app1", "app2"},
				"envs/bar/stage": []string{"app1", "app2"},
			},
		},
		{
			"empty list prioritised",
			EnvAppMap{
				"envs/bar":      nil,
				"envs/bar/prod": []string{"app1", "app2"},
				"envs/foo":      []string{},
				"envs/foo/prod": []string{"app3"},
			},
			EnvAppMap{
				"envs/bar/prod":  []string{},
				"envs/bar/stage": []string{},
				"envs/foo/prod":  []string{},
				"envs/foo/stage": []string{},
			},
		},
	}

	for _, tt := range tests {
		out := g.collectEnvironments(tt.in)
		if !compareEnvAppMap(out, tt.out) {
			t.Errorf("%s:\n  got  %v\n  want %v", tt.name, out, tt.out)
		}
	}
}

func Test_getEnvironmentsUnderRoot(t *testing.T) {
	g := NewWithDefaults()

	// Set up test environments
	g.environments = map[string]*Environment{
		"envs/dev":         {},
		"envs/staging":     {},
		"envs/prod":        {},
		"envs/team-a/dev":  {},
		"envs/team-b/prod": {},
		"envs/something":   {},
	}

	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{
			name:     "exact match",
			path:     "envs/dev",
			expected: []string{"envs/dev"},
		},
		{
			name:     "prefix match",
			path:     "envs",
			expected: []string{"envs/dev", "envs/staging", "envs/prod", "envs/team-a/dev", "envs/team-b/prod", "envs/something"},
		},
		{
			name:     "prefix match with separator",
			path:     "envs/",
			expected: []string{"envs/dev", "envs/staging", "envs/prod", "envs/team-a/dev", "envs/team-b/prod", "envs/something"},
		},
		{
			name:     "nested prefix match",
			path:     "envs/team-a",
			expected: []string{"envs/team-a/dev"},
		},
		{
			name:     "no match",
			path:     "other/path",
			expected: []string{},
		},
		{
			name:     "similar prefix no match",
			path:     "envs-other",
			expected: []string{},
		},
		{
			name:     "partial match but not prefix",
			path:     "dev",
			expected: []string{},
		},
		{
			name:     "case sensitive no match",
			path:     "ENVS/DEV",
			expected: []string{},
		},
		{
			name:     "partial prefix no match (prevents envs/some matching envs/something)",
			path:     "envs/some",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.getEnvironmentsUnderRoot(tt.path)
			// Sort both slices for comparison
			sort.Strings(result)
			sort.Strings(tt.expected)
			// Use len comparison to handle nil vs empty slice
			if len(result) != len(tt.expected) {
				t.Errorf("getEnvironmentsUnderRoot(%q) = %v; want %v", tt.path, result, tt.expected)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("getEnvironmentsUnderRoot(%q) = %v; want %v", tt.path, result, tt.expected)
					return
				}
			}
		})
	}
}

func compareEnvAppMap(left, right EnvAppMap) bool {
	if len(left) != len(right) {
		return false
	}
	for leftEnv, leftApps := range left {
		if rightApps, ok := right[leftEnv]; !ok {
			return false
		} else {
			if len(leftApps) != len(rightApps) {
				return false
			}
			sort.Strings(leftApps)
			sort.Strings(rightApps)
			for i, leftApp := range leftApps {
				if leftApp != rightApps[i] {
					return false
				}
			}
		}
	}
	return true
}

func Test_buildYttGlobeData(t *testing.T) {
	tests := []struct {
		name          string
		gitRepoBranch string
		gitRepoURL    string
	}{
		{
			name:          "basic config",
			gitRepoBranch: "main",
			gitRepoURL:    "https://github.com/example/repo.git",
		},
		{
			name:          "empty values",
			gitRepoBranch: "",
			gitRepoURL:    "",
		},
		{
			name:          "special characters in branch",
			gitRepoBranch: "feature/test-branch_123",
			gitRepoURL:    "git@github.com:org/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithDefaults()
			g.GitRepoBranch = tt.gitRepoBranch
			g.GitRepoURL = tt.gitRepoURL

			data := g.buildYttGlobeData()

			if data.GitRepoBranch != tt.gitRepoBranch {
				t.Errorf("buildYttGlobeData() GitRepoBranch = %v, want %v", data.GitRepoBranch, tt.gitRepoBranch)
			}
			if data.GitRepoURL != tt.gitRepoURL {
				t.Errorf("buildYttGlobeData() GitRepoURL = %v, want %v", data.GitRepoURL, tt.gitRepoURL)
			}
		})
	}
}

func Test_encodeConfigAsYtt(t *testing.T) {
	tests := []struct {
		name              string
		data              *YttGlobeData
		wantErr           bool
		wantHeader        bool
		wantSubstrings    []string
		notWantSubstrings []string
	}{
		{
			name: "basic config",
			data: &YttGlobeData{
				GitRepoBranch: "main",
				GitRepoURL:    "https://github.com/example/repo.git",
			},
			wantErr:    false,
			wantHeader: true,
			wantSubstrings: []string{
				"myks:",
				"gitRepoBranch: main",
				"gitRepoUrl: https://github.com/example/repo.git",
			},
		},
		{
			name: "empty values",
			data: &YttGlobeData{
				GitRepoBranch: "",
				GitRepoURL:    "",
			},
			wantErr:    false,
			wantHeader: true,
			wantSubstrings: []string{
				"myks:",
				"gitRepoBranch:",
				"gitRepoUrl:",
			},
		},
		{
			name: "special characters",
			data: &YttGlobeData{
				GitRepoBranch: "feature/test-branch_123",
				GitRepoURL:    "git@github.com:org/repo.git",
			},
			wantErr:    false,
			wantHeader: true,
			wantSubstrings: []string{
				"gitRepoBranch: feature/test-branch_123",
				"gitRepoUrl: git@github.com:org/repo.git",
			},
		},
		{
			name:       "nil data",
			data:       nil,
			wantErr:    false,
			wantHeader: true,
			wantSubstrings: []string{
				"myks: null",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := encodeConfigAsYtt(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("encodeConfigAsYtt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if tt.wantHeader && !strings.HasPrefix(result, "#@data/values-schema\n---\n") {
				t.Error("encodeConfigAsYtt() result should start with ytt data/values-schema header")
			}

			for _, substr := range tt.wantSubstrings {
				if !strings.Contains(result, substr) {
					t.Errorf("encodeConfigAsYtt() result should contain %q, got:\n%s", substr, result)
				}
			}

			for _, substr := range tt.notWantSubstrings {
				if strings.Contains(result, substr) {
					t.Errorf("encodeConfigAsYtt() result should not contain %q, got:\n%s", substr, result)
				}
			}
		})
	}
}

func Test_dumpConfigAsYaml(t *testing.T) {
	tests := []struct {
		name          string
		gitRepoBranch string
		gitRepoURL    string
		wantErr       bool
	}{
		{
			name:          "basic config",
			gitRepoBranch: "main",
			gitRepoURL:    "https://github.com/example/repo.git",
			wantErr:       false,
		},
		{
			name:          "empty values",
			gitRepoBranch: "",
			gitRepoURL:    "",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			g := NewWithDefaults()
			g.RootDir = tmpDir
			g.GitRepoBranch = tt.gitRepoBranch
			g.GitRepoURL = tt.gitRepoURL

			configFile, err := g.dumpConfigAsYaml()
			if (err != nil) != tt.wantErr {
				t.Errorf("dumpConfigAsYaml() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify file was created
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Errorf("dumpConfigAsYaml() config file was not created: %s", configFile)
				return
			}

			// Verify file path
			expectedPath := filepath.Join(tmpDir, g.ServiceDirName, g.TempDirName, g.MyksDataFileName)
			if configFile != expectedPath {
				t.Errorf("dumpConfigAsYaml() configFile = %v, want %v", configFile, expectedPath)
			}

			// Verify content matches what encodeConfigAsYtt produces
			content, err := os.ReadFile(configFile)
			if err != nil {
				t.Fatalf("Failed to read config file: %v", err)
			}

			expectedContent, _ := encodeConfigAsYtt(g.buildYttGlobeData())
			if string(content) != expectedContent {
				t.Errorf("dumpConfigAsYaml() file content mismatch\ngot:\n%s\nwant:\n%s", content, expectedContent)
			}
		})
	}
}

func Test_dumpConfigAsYaml_directoryCreation(t *testing.T) {
	tmpDir := t.TempDir()

	g := NewWithDefaults()
	g.RootDir = tmpDir
	g.ServiceDirName = "custom-service"
	g.TempDirName = "custom-temp"

	expectedDir := filepath.Join(tmpDir, "custom-service", "custom-temp")

	// Verify directory doesn't exist before
	if _, err := os.Stat(expectedDir); !os.IsNotExist(err) {
		t.Fatalf("Directory should not exist before dumpConfigAsYaml(): %s", expectedDir)
	}

	_, err := g.dumpConfigAsYaml()
	if err != nil {
		t.Fatalf("dumpConfigAsYaml() unexpected error: %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(expectedDir)
	if err != nil {
		t.Fatalf("Directory should exist after dumpConfigAsYaml(): %v", err)
	}

	if !info.IsDir() {
		t.Error("Created path should be a directory")
	}
}

func Test_dumpConfigAsYaml_filePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	g := NewWithDefaults()
	g.RootDir = tmpDir

	configFile, err := g.dumpConfigAsYaml()
	if err != nil {
		t.Fatalf("dumpConfigAsYaml() unexpected error: %v", err)
	}

	info, err := os.Stat(configFile)
	if err != nil {
		t.Fatalf("Failed to stat config file: %v", err)
	}

	// Verify file permissions (0600 = rw-------)
	expectedPerm := os.FileMode(0o600)
	actualPerm := info.Mode().Perm()
	if actualPerm != expectedPerm {
		t.Errorf("dumpConfigAsYaml() file permissions = %v, want %v", actualPerm, expectedPerm)
	}
}

func Test_dumpConfigAsYaml_overwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()

	g := NewWithDefaults()
	g.RootDir = tmpDir
	g.GitRepoBranch = "initial-branch"
	g.GitRepoURL = "https://github.com/initial/repo.git"

	// First call
	configFile, err := g.dumpConfigAsYaml()
	if err != nil {
		t.Fatalf("First dumpConfigAsYaml() unexpected error: %v", err)
	}

	// Modify values
	g.GitRepoBranch = "updated-branch"
	g.GitRepoURL = "https://github.com/updated/repo.git"

	// Second call
	configFile2, err := g.dumpConfigAsYaml()
	if err != nil {
		t.Fatalf("Second dumpConfigAsYaml() unexpected error: %v", err)
	}

	if configFile != configFile2 {
		t.Errorf("dumpConfigAsYaml() should return same path, got %v and %v", configFile, configFile2)
	}

	// Verify content was updated
	content, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)
	if strings.Contains(contentStr, "initial-branch") {
		t.Error("dumpConfigAsYaml() should have overwritten file with new values")
	}
	if !strings.Contains(contentStr, "updated-branch") {
		t.Error("dumpConfigAsYaml() should contain updated branch value")
	}
}
