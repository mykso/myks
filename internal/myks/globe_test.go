package myks

import (
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
			t.Errorf("AddBaseDirToEnvPath(%s) = %s; want %s", tt.in, out, tt.out)
		}
	}
}
