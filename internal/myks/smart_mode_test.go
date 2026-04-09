package myks

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"testing"

	"github.com/creasty/defaults"
)

func getChanges(changedFilePaths []string, regExps ...string) (matches1, matches2 []string) {
	for _, expr := range regExps {
		re := regexp.MustCompile(expr)
		for _, line := range changedFilePaths {
			matches := re.FindStringSubmatch(line)
			if matches != nil {
				switch len(matches) {
				case 1:
					matches1 = append(matches1, matches[0])
				case 2:
					matches1 = append(matches1, matches[1])
				default:
					matches1 = append(matches1, matches[1])
					matches2 = append(matches2, matches[2])
				}
			}
		}
	}
	return matches1, matches2
}

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

func TestGlobe_findPrototypeUsage(t *testing.T) {
	type args struct {
		prototypes []string
		globe      Globe
	}
	g1 := createGlobe(t)
	g1.environments = map[string]*Environment{
		"envs/env1": {
			g:   g1,
			cfg: &g1.Config,
			ID:  "env1",
			foundApplications: map[string]string{
				"app1": "proto1",
				"app2": "proto2",
			},
		},
	}
	g2 := createGlobe(t)
	g2.environments = map[string]*Environment{
		"envs/env1": {
			g:   g2,
			cfg: &g2.Config,
			ID:  "env1",
			foundApplications: map[string]string{
				"app1": "proto1",
				"app2": "proto2/subproto1",
			},
		},
	}
	g3 := createGlobe(t)
	g3.environments = map[string]*Environment{
		"envs/env1": {
			g:   g3,
			cfg: &g3.Config,
			ID:  "env1",
			foundApplications: map[string]string{
				"app1": "proto1",
				"app2": "proto1",
			},
		},
		"envs/env2": {
			g:   g3,
			cfg: &g3.Config,
			ID:  "env2",
			foundApplications: map[string]string{
				"app3": "proto1",
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
				"envs/env1": {"app1"},
			},
		},
		{
			"prototype ref",
			args{
				[]string{"proto1", "proto2/subproto1"},
				*g2,
			},
			EnvAppMap{
				"envs/env1": {"app1", "app2"},
			},
		},
		{
			"duplicates",
			args{
				[]string{"proto1", "proto2"},
				*g3,
			},
			EnvAppMap{
				"envs/env1": {"app1", "app2"},
				"envs/env2": {"app3"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envAppsMap := tt.args.globe.findPrototypeUsage(tt.args.prototypes, "")

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
			Dir: "envs/env1",
			g:   g,
			cfg: &g.Config,
			ID:  "env1-id",
			foundApplications: map[string]string{
				"app1":  "app1",
				"app2":  "app2",
				"app22": "app2", // app22 also uses prototype app2
			},
		},
		"envs/env2": {
			Dir: "envs/env2",
			g:   g,
			cfg: &g.Config,
			ID:  "env2",
			foundApplications: map[string]string{
				"app3": "app3",
				"app2": "app2",
			},
		},
	}
	renderedEnvApps := EnvAppMap{
		"env1-id": {"app1", "app2", "app22"},
		"env2":    {"app2", "app3"},
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
			"change to prototypes _vendir",
			ChangedFiles{"prototypes/_vendir/file1": "M"},
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
			ChangedFiles{"envs/env1/_apps/app1/app-data.variable.yaml": "M"},
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
				"envs/env1": {"app2", "app22"},
				"envs/env2": {"app2", "app3"},
			},
		},
		{
			"missing rendered apps",
			ChangedFiles{},
			EnvAppMap{
				"env1-id": {"app1"},
				"env2":    {"app2"},
			},
			EnvAppMap{
				"envs/env1": {"app2", "app22"},
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
		{
			// Regression guard: verifies pruneNestedEnvAppMap is called in runSmartMode.
			// Without the call, the result would be {envs: nil, envs/env1: [app1]}.
			"root _env wildcard prunes nested env entries",
			ChangedFiles{
				"envs/_env/argocd/some-file.yaml":             "?",
				"envs/env1/_apps/app1/app-data.variable.yaml": "M",
			},
			renderedEnvApps,
			EnvAppMap{
				"envs": nil,
			},
		},
		{
			"changes in rendered envs",
			ChangedFiles{
				"rendered/envs/env1-id/app1/some-file.yaml": "M",
				"rendered/envs/env2/app3/some-file.yaml":    "?",
			},
			renderedEnvApps,
			EnvAppMap{
				"envs/env1": {"app1"},
				"envs/env2": {"app3"},
			},
		},
		{
			"changes in rendered envs with missing apps",
			ChangedFiles{
				"rendered/envs/env1-id/app1/some-file.yaml": "M",
			},
			EnvAppMap{
				"env1-id": {"app1"},
				"env2":    {"app2"},
			},
			EnvAppMap{
				"envs/env1": {"app1", "app2", "app22"},
				"envs/env2": {"app3"},
			},
		},
		{
			"changes in rendered argocd apps",
			ChangedFiles{
				"rendered/argocd/env1-id/app-app1.yaml": "M",
			},
			renderedEnvApps,
			EnvAppMap{
				"envs/env1": {"app1"},
			},
		},
		{
			"changes in environment-specific prototype configuration",
			ChangedFiles{
				"envs/env1/_proto/app1/ytt/some-file.yaml": "M",
			},
			renderedEnvApps,
			EnvAppMap{
				"envs/env1": {"app1"},
			},
		},
		{
			"changes in environment-specific prototype app data",
			ChangedFiles{
				"envs/env1/_proto/app2/app-data.ytt.yaml": "M",
			},
			renderedEnvApps,
			EnvAppMap{
				"envs/env1": {"app2", "app22"},
			},
		},
		{
			"changes in parent environment-specific prototype affects child environments",
			ChangedFiles{
				"envs/_proto/app2/ytt/some-file.yaml": "M",
			},
			renderedEnvApps,
			EnvAppMap{
				"envs/env1": {"app2", "app22"},
				"envs/env2": {"app2"},
			},
		},
		{
			"changes in environment-specific prototype affects multiple applications using same prototype",
			ChangedFiles{
				"envs/env1/_proto/app2/ytt/some-file.yaml": "M",
			},
			renderedEnvApps,
			EnvAppMap{
				"envs/env1": {"app2", "app22"},
			},
		},
		{
			"changes in application-specific lib directory",
			ChangedFiles{
				"envs/env1/_apps/app1/lib/some-lib-file.yaml": "M",
			},
			renderedEnvApps,
			EnvAppMap{
				"envs/env1": {"app1"},
			},
		},
		{
			"changes in prototype-specific lib directory",
			ChangedFiles{
				"prototypes/app2/lib/some-lib-file.yaml": "M",
			},
			renderedEnvApps,
			EnvAppMap{
				"envs/env1": {"app2", "app22"},
				"envs/env2": {"app2"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			for env, apps := range tt.rendered {
				for _, app := range apps {
					dir := filepath.Join(tmpDir, g.RenderedEnvsDir, env, app)
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

func Test_squashEnvPaths(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			"ancestor dominates descendant",
			[]string{"envs/prod", "envs/prod/eu"},
			[]string{"envs/prod"},
		},
		{
			"siblings are kept",
			[]string{"envs/prod/eu", "envs/prod/us", "envs/staging"},
			[]string{"envs/prod/eu", "envs/prod/us", "envs/staging"},
		},
		{
			"dedup and squash",
			[]string{"envs/prod", "envs/prod", "envs/prod/eu/a"},
			[]string{"envs/prod"},
		},
		{
			"empty input",
			[]string{},
			[]string{},
		},
		{
			"single path",
			[]string{"envs/prod"},
			[]string{"envs/prod"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := squashEnvPaths(tt.in)
			sort.Strings(got)
			sort.Strings(tt.want)
			assertEqual(t, got, tt.want)
		})
	}
}

func Test_pruneNestedEnvAppMap(t *testing.T) {
	tests := []struct {
		name string
		in   EnvAppMap
		want EnvAppMap
	}{
		{
			"descendant with apps pruned under wildcard ancestor",
			EnvAppMap{"envs/prod": nil, "envs/prod/eu": {"app1"}},
			EnvAppMap{"envs/prod": nil},
		},
		{
			"no wildcard — nothing pruned",
			EnvAppMap{"envs/prod": {"app1"}, "envs/prod/eu": {"app2"}},
			EnvAppMap{"envs/prod": {"app1"}, "envs/prod/eu": {"app2"}},
		},
		{
			"unrelated env not pruned",
			EnvAppMap{"envs/prod": nil, "envs/staging/eu": {"app1"}},
			EnvAppMap{"envs/prod": nil, "envs/staging/eu": {"app1"}},
		},
		{
			"wildcard itself not pruned",
			EnvAppMap{"envs/prod": nil},
			EnvAppMap{"envs/prod": nil},
		},
		{
			"multiple wildcards",
			EnvAppMap{
				"envs/prod":    nil,
				"envs/staging": nil,
				"envs/prod/eu": {"app1"},
				"envs/qa":      {"app2"},
			},
			EnvAppMap{"envs/prod": nil, "envs/staging": nil, "envs/qa": {"app2"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pruneNestedEnvAppMap(tt.in)
			assertEqual(t, tt.in, tt.want)
		})
	}
}

func createGlobe(t *testing.T) *Globe {
	g := &Globe{}
	if err := defaults.Set(&g.Config); err != nil {
		t.Errorf("failed to create Globe object")
	}
	return g
}
