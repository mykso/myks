package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const unsupportedOutputFormatErr = "unsupported output format"

func Test_validateOutputFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr string
	}{
		{name: "text is valid", format: inspectOutputText},
		{name: "json is valid", format: inspectOutputJSON},
		{name: "yaml is invalid", format: "yaml", wantErr: unsupportedOutputFormatErr},
		{name: "empty string is invalid", format: "", wantErr: unsupportedOutputFormatErr},
		{name: "xml is invalid", format: "xml", wantErr: unsupportedOutputFormatErr},
		{name: "JSON uppercase is invalid", format: "JSON", wantErr: unsupportedOutputFormatErr},
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
