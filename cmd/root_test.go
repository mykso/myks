package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigFlagValue(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no flag", []string{"render", "dev"}, ""},
		{"separate value", []string{"--config", "custom.yaml", "render"}, "custom.yaml"},
		{"equals value", []string{"render", "--config=custom.yaml"}, "custom.yaml"},
		{"last one wins", []string{"--config", "a.yaml", "--config=b.yaml"}, "b.yaml"},
		{"missing value", []string{"render", "--config"}, ""},
		{"plugin args ignored", []string{"plugin", "--", "--config", "plugin.yaml"}, ""},
		{"flag before plugin args", []string{"--config=a.yaml", "plugin", "--", "--config=b.yaml"}, "a.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, configFlagValue(tt.args))
		})
	}
}
