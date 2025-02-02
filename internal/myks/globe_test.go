package myks

import (
	"sort"
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
