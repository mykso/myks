package myks

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/creasty/defaults"
)

func Test_getChanges(t *testing.T) {
	type args struct {
		diff         []string
		regExPattern string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			"happy path",
			args{
				[]string{
					"path1/file1",
					"path1/file2",
				},
				"^path1/(.*)$",
			},
			[]string{
				"file1",
				"file2",
			},
		},
		{
			"no capture group",
			args{
				[]string{
					"path1/file1",
					"path1/file2",
				},
				"^path1/.*$",
			},
			[]string{
				"path1/file1",
				"path1/file2",
			},
		},
		{
			"no match",
			args{
				[]string{
					"nothing-to-match",
				},
				"^path1/.*$",
			},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := getChanges(tt.args.diff, tt.args.regExPattern); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkFileChanged(t *testing.T) {
	type args struct {
		changedFiles []string
		regExps      []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"happy path", args{[]string{"path1/file1"}, []string{"^path1/(.*)$"}}, true},
		{"no match", args{[]string{"path1/file1"}, []string{"no-match"}}, false},
		{"empty", args{[]string{}, []string{"no-match"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkFileChanged(tt.args.changedFiles, tt.args.regExps...); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGlobe_findPrototypeUsage(t *testing.T) {
	type args struct {
		prototypes []string
		globe      Globe
	}
	g1 := createGlobe(t)
	g1.environments = map[string]*Environment{
		"env1": {
			g:  g1,
			Id: "env1",
			foundApplications: map[string]string{
				"app1": "proto1",
				"app2": "proto2",
			},
		},
	}
	g2 := createGlobe(t)
	g2.environments = map[string]*Environment{
		"env1": {
			g:  g2,
			Id: "env1",
			foundApplications: map[string]string{
				"app1":      "proto1",
				"root/app2": "proto2",
			},
		},
	}
	g3 := createGlobe(t)
	g3.environments = map[string]*Environment{
		"env1": {
			g:  g3,
			Id: "env1",
			foundApplications: map[string]string{
				"app1": "proto1",
			},
		},
		"env2": {
			g:  g3,
			Id: "env2",
			foundApplications: map[string]string{
				"app1": "proto1",
			},
		},
	}

	tests := []struct {
		name           string
		args           args
		wantEnvAppsMap EnvAppMap
	}{
		{
			"happy path",
			args{
				[]string{"proto1"},
				*g1,
			},
			EnvAppMap{
				"env1": {"app1"},
			},
		},
		{
			"prototype ref",
			args{
				[]string{"proto1", "proto2"},
				*g2,
			},
			EnvAppMap{
				"env1": {"app1", "root/app2"},
			},
		},
		{
			"duplicates",
			args{
				[]string{"proto1", "proto2"},
				*g3,
			},
			EnvAppMap{
				"env1": {"app1"},
				"env2": {"app1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envAppsMap := tt.args.globe.findPrototypeUsage(tt.args.prototypes)

			for _, apps := range envAppsMap {
				sort.Strings(apps)
			}

			for _, apps := range tt.wantEnvAppsMap {
				sort.Strings(apps)
			}

			assertEqual(t, envAppsMap, tt.wantEnvAppsMap)
		})
	}
}

func TestGlobe_runSmartMode(t *testing.T) {
	g := createGlobe(t)
	g.environments = map[string]*Environment{
		"envs/env1": {
			g:  g,
			Id: "env1",
			foundApplications: map[string]string{
				"app1": "app1",
				"app2": "app2",
			},
		},
		"envs/env2": {
			g:  g,
			Id: "env2",
			foundApplications: map[string]string{
				"app3": "app3",
				"app2": "app2",
			},
		},
	}
	renderedEnvApps := EnvAppMap{
		"env1": {"app1", "app2"},
		"env2": {"app2", "app3"},
	}
	tests := []struct {
		name         string
		changedFiles ChangedFiles
		rendered     map[string][]string
		envAppsMap   EnvAppMap
	}{
		{
			"change to global lib",
			ChangedFiles{"lib/file1": "M"},
			renderedEnvApps,
			EnvAppMap{
				g.EnvironmentBaseDir: nil,
			},
		},
		{
			"change to prototype",
			ChangedFiles{"prototypes/app1/app-data.ytt.yaml": "M"},
			renderedEnvApps,
			EnvAppMap{
				"envs/env1": {"app1"},
			},
		},
		{
			"change to app",
			ChangedFiles{"envs/env1/_apps/app1/app-data.ytt.yaml": "M"},
			renderedEnvApps,
			EnvAppMap{
				"envs/env1": {"app1"},
			},
		},
		{
			"change to env",
			ChangedFiles{
				"envs/env1/env-data.ytt.yaml":            "M",
				"envs/env1/_apps/app1/app-data.ytt.yaml": "M",
			},
			renderedEnvApps,
			EnvAppMap{
				"envs/env1": nil,
			},
		},
		{
			"env deletion",
			ChangedFiles{"envs/env1/env-data.ytt.yaml": "D"},
			renderedEnvApps,
			EnvAppMap{
				"envs/env1": nil,
			},
		},
		{
			"changes to all multiple envs and apps",
			ChangedFiles{
				"prototypes/app2/app-data.ytt.yaml":       "M",
				"envs/env2/_apps/app3/ytt/some-file.yaml": "M",
			},
			renderedEnvApps,
			EnvAppMap{
				"envs/env1": {"app2"},
				"envs/env2": {"app2", "app3"},
			},
		},
		{
			"missing rendered apps",
			ChangedFiles{},
			EnvAppMap{
				"env1": {"app1"},
				"env2": {"app2"},
			},
			EnvAppMap{
				"envs/env1": {"app2"},
				"envs/env2": {"app3"},
			},
		},
		{
			"changes in _env",
			ChangedFiles{
				"envs/env1/_env/argocd/some-file.yaml": "M",
				"envs/env2/_env/ytt/some-file.yaml":    "?",
			},
			renderedEnvApps,
			EnvAppMap{
				"envs/env1": nil,
				"envs/env2": nil,
			},
		},
		{
			"changes in root _env",
			ChangedFiles{
				"envs/_env/argocd/some-file.yaml": "?",
			},
			renderedEnvApps,
			EnvAppMap{
				"envs": nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			for env, apps := range tt.rendered {
				for _, app := range apps {
					dir := filepath.Join(tmpDir, g.RenderedDir, "envs", env, app)
					if err := createDirectory(dir); err != nil {
						t.Errorf("failed to create directory %s", dir)
					}
				}
			}

			g.RootDir = tmpDir

			defer func() {
				err := os.RemoveAll(tmpDir)
				if err != nil {
					t.Errorf("failed to remove temporary directory %s", tmpDir)
				}
			}()

			envAppsMap := g.runSmartMode(tt.changedFiles)
			for _, apps := range envAppsMap {
				sort.Strings(apps)
			}
			for _, apps := range tt.envAppsMap {
				sort.Strings(apps)
			}
			assertEqual(t, envAppsMap, tt.envAppsMap)
		})
	}
}

func createGlobe(t *testing.T) *Globe {
	g := &Globe{}
	if err := defaults.Set(g); err != nil {
		t.Errorf("failed to create Globe object")
	}
	return g
}
