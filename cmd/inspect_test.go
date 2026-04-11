package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_validateOutputFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr string
	}{
		{name: "text is valid", format: "text"},
		{name: "json is valid", format: "json"},
		{name: "yaml is invalid", format: "yaml", wantErr: "unsupported output format"},
		{name: "empty string is invalid", format: "", wantErr: "unsupported output format"},
		{name: "xml is invalid", format: "xml", wantErr: "unsupported output format"},
		{name: "JSON uppercase is invalid", format: "JSON", wantErr: "unsupported output format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.PersistentFlags().StringP("output", "o", inspectOutputText, "")
			require.NoError(t, cmd.PersistentFlags().Set("output", tt.format))

			err := validateOutputFormat(cmd)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
