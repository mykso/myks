package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mykso/myks/internal/myks"
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

func Test_groupRedundant(t *testing.T) {
	const proto, foo, bar, baz, schema = "prototypes/p.yaml", "envs/foo/p.yaml", "envs/foo/bar/p.yaml", "envs/foo/bar/baz/p.yaml", "prototypes/schema.yaml"
	groups := groupRedundant([]myks.InspectRedundantOverride{
		{Path: "a", File: foo, Value: 1, InheritedFrom: proto},
		{Path: "b", File: foo, Value: 2, InheritedFrom: proto},
		{Path: "c", File: foo, Value: 3, InheritedFrom: proto},
		{Path: "a", File: bar, Value: 1, InheritedFrom: proto},
		{Path: "b", File: bar, Value: 2, InheritedFrom: proto},
		{Path: "d", File: bar, Value: 4, InheritedFrom: ""},
		{Path: "c", File: baz, Value: 5, InheritedFrom: bar},
		{Path: "e", File: proto, Value: 6, InheritedFrom: schema},
	})
	assert.Equal(t, []redundantGroup{
		{keptIn: proto, repeatedIn: []string{foo, bar}, keys: []myks.InspectRedundantOverride{
			{Path: "a", File: foo, Value: 1, InheritedFrom: proto},
			{Path: "b", File: foo, Value: 2, InheritedFrom: proto},
		}},
		{keptIn: proto, repeatedIn: []string{foo}, keys: []myks.InspectRedundantOverride{
			{Path: "c", File: foo, Value: 3, InheritedFrom: proto},
		}},
		{keptIn: "", repeatedIn: []string{bar}, keys: []myks.InspectRedundantOverride{
			{Path: "d", File: bar, Value: 4, InheritedFrom: ""},
		}},
		// bar changes c, so baz repeats another value in another chain.
		{keptIn: bar, repeatedIn: []string{baz}, keys: []myks.InspectRedundantOverride{
			{Path: "c", File: baz, Value: 5, InheritedFrom: bar},
		}},
		{keptIn: schema, repeatedIn: []string{proto}, keys: []myks.InspectRedundantOverride{
			{Path: "e", File: proto, Value: 6, InheritedFrom: schema},
		}},
	}, groups)
}
