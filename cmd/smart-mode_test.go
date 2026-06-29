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
		{name: outputFormatText, raw: outputFormatText, want: outputFormatText},
		{name: outputFormatJSON, raw: outputFormatJSON, want: outputFormatJSON},

		// Case insensitivity
		{name: "TEXT uppercase", raw: "TEXT", want: outputFormatText},
		{name: "JSON uppercase", raw: "JSON", want: outputFormatJSON},
		{name: "Json mixed case", raw: "Json", want: outputFormatJSON},
		{name: "Text mixed case", raw: "Text", want: outputFormatText},

		// Whitespace trimming
		{name: "text with spaces", raw: "  text  ", want: outputFormatText},
		{name: "json with spaces", raw: " json ", want: outputFormatJSON},

		// Legacy truthy -> text
		{name: AnnotationTrue, raw: AnnotationTrue, want: outputFormatText},
		{name: "TRUE", raw: "TRUE", want: outputFormatText},
		{name: "True", raw: "True", want: outputFormatText},
		{name: "1", raw: "1", want: outputFormatText},
		{name: booleanYes, raw: booleanYes, want: outputFormatText},
		{name: "YES", raw: "YES", want: outputFormatText},

		// Legacy falsey -> disabled
		{name: annotationFalse, raw: annotationFalse, want: ""},
		{name: "FALSE", raw: "FALSE", want: ""},
		{name: "False", raw: "False", want: ""},
		{name: "0", raw: "0", want: ""},
		{name: booleanNo, raw: booleanNo, want: ""},
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
