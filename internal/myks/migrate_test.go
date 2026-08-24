package myks

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasYttLogicRe(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"plain values header", "#@data/values\n---\nfoo: bar\n", false},
		{"schema header with overlay", "#@data/values-schema\n#@overlay/match-child-defaults missing_ok=True\n---\nfoo: bar\n", false},
		{"load directive", "#@ load(\"@myks:data.lib.yaml\", \"env_data\")\n#@data/values-schema\n---\n", true},
		{"inline expression", "#@data/values-schema\n---\nenvId: #@ env_data.environment.id\n", true},
		{"schema default annotation", "#@data/values-schema\n---\n#@schema/default [\"x\"]\nitems: ['']\n", true},
		{"plain comment", "#! just a comment\n#@data/values\n---\nfoo: bar\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, hasYttLogicRe.MatchString(tt.content))
		})
	}
}

func TestMergeValues(t *testing.T) {
	base := map[string]any{
		"a": map[string]any{"x": 1, "y": []any{1, 2}},
		"b": "keep",
	}
	over := map[string]any{
		"a": map[string]any{"y": []any{3}, "z": nil},
		"c": true,
	}
	got := mergeValues(base, over)
	want := map[string]any{
		"a": map[string]any{"x": 1, "y": []any{3}, "z": nil},
		"b": "keep",
		"c": true,
	}
	assert.Equal(t, want, got)
	// Inputs stay untouched.
	assert.Equal(t, []any{1, 2}, base["a"].(map[string]any)["y"])
}

func TestDiffValueMaps(t *testing.T) {
	got := map[string]any{
		"same":   "v",
		"nested": map[string]any{"keep": 1, "change": "old"},
		"extra":  "converted-only",
	}
	want := map[string]any{
		"same":    "v",
		"nested":  map[string]any{"keep": 1, "change": "new", "add": true},
		"missing": []any{"x"},
	}
	var extra, lists []string
	patch := diffValueMaps(got, want, "", &extra, &lists)
	assert.Equal(t, map[string]any{
		"nested":  map[string]any{"change": "new", "add": true},
		"missing": []any{"x"},
	}, patch)
	assert.Equal(t, []string{".extra"}, extra)
	assert.Empty(t, lists)
}

func TestKclScalar(t *testing.T) {
	tests := []struct {
		value any
		want  string
	}{
		{nil, "None"},
		{true, "True"},
		{false, "False"},
		{"text", `"text"`},
		{"with \"quotes\"", `"with \"quotes\""`},
		{42, "42"},
		{int64(-7), "-7"},
		{1.5, "1.5"},
		{1.0, "1.0"}, // integral floats keep the dot to stay floats in KCL
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, kclScalar(tt.value))
	}
}

func TestWriteKclEntries(t *testing.T) {
	values := map[string]any{
		"plain":      "v",
		"weird-key":  true,
		"emptyList":  []any{},
		"list":       []any{map[string]any{"name": "a"}, 2},
		"nested":     map[string]any{"inner": 1},
		"emptyDict":  map[string]any{},
		"nullValue":  nil,
		"floatValue": 2.0,
	}

	assign := &strings.Builder{}
	writeKclEntries(assign, values, 0, false)
	assert.Equal(t, `emptyDict = {}
emptyList = []
floatValue = 2.0
list = [
    {
        name = "a"
    }
    2
]
nested = {
    inner = 1
}
nullValue = None
plain = "v"
"weird-key" = True
`, assign.String())

	merge := &strings.Builder{}
	writeKclEntries(merge, map[string]any{"nested": map[string]any{"inner": 1}, "scalar": "v"}, 0, true)
	assert.Equal(t, `nested: {
    inner = 1
}
scalar = "v"
`, merge.String())
}

func TestIsKclIdentifier(t *testing.T) {
	assert.True(t, isKclIdentifier("envs"))
	assert.True(t, isKclIdentifier("central_forwarder"))
	assert.True(t, isKclIdentifier("_x1"))
	assert.False(t, isKclIdentifier("central-forwarder"))
	assert.False(t, isKclIdentifier("1abc"))
	assert.False(t, isKclIdentifier("schema"))
	assert.False(t, isKclIdentifier("str"))
	assert.False(t, isKclIdentifier(""))
}
