package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_normalizeOnlyPrint(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		// Disabled (empty)
		{name: "empty string", raw: "", want: ""},
		{name: "whitespace only", raw: "  ", want: ""},

		// Canonical values
		{name: "text", raw: "text", want: "text"},
		{name: "json", raw: "json", want: "json"},

		// Case insensitivity
		{name: "TEXT uppercase", raw: "TEXT", want: "text"},
		{name: "JSON uppercase", raw: "JSON", want: "json"},
		{name: "Json mixed case", raw: "Json", want: "json"},
		{name: "Text mixed case", raw: "Text", want: "text"},

		// Whitespace trimming
		{name: "text with spaces", raw: "  text  ", want: "text"},
		{name: "json with spaces", raw: " json ", want: "json"},

		// Legacy truthy -> text
		{name: "true", raw: "true", want: "text"},
		{name: "TRUE", raw: "TRUE", want: "text"},
		{name: "True", raw: "True", want: "text"},
		{name: "1", raw: "1", want: "text"},
		{name: "yes", raw: "yes", want: "text"},
		{name: "YES", raw: "YES", want: "text"},

		// Legacy falsey -> disabled
		{name: "false", raw: "false", want: ""},
		{name: "FALSE", raw: "FALSE", want: ""},
		{name: "False", raw: "False", want: ""},
		{name: "0", raw: "0", want: ""},
		{name: "no", raw: "no", want: ""},
		{name: "NO", raw: "NO", want: ""},

		// Invalid values
		{name: "invalid value", raw: "yaml", wantErr: `invalid value "yaml" for --smart-mode.only-print`},
		{name: "typo", raw: "jsn", wantErr: `invalid value "jsn" for --smart-mode.only-print`},
		{name: "numeric non-boolean", raw: "2", wantErr: `invalid value "2" for --smart-mode.only-print`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeOnlyPrint(tt.raw)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
