package cmd

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mykso/myks/internal/myks"
)

type jsonSchema struct {
	Properties map[string]jsonSchema `json:"properties"`
}

// TestConfigSchemaInSync fails when a config key is added in code but not in schema/myks.schema.json, or the reverse.
func TestConfigSchemaInSync(t *testing.T) {
	raw, err := os.ReadFile("../schema/myks.schema.json")
	require.NoError(t, err)
	var schema jsonSchema
	require.NoError(t, json.Unmarshal(raw, &schema))

	var conventions []string
	rt := reflect.TypeFor[myks.Config]()
	for i := 0; i < rt.NumField(); i++ {
		if tag := rt.Field(i).Tag.Get("mapstructure"); tag != "" {
			conventions = append(conventions, tag)
		}
	}
	assert.ElementsMatch(t, conventions, keys(schema.Properties["naming-conventions"].Properties))

	// Every persistent flag is also a config key; dotted flags are nested objects.
	want := []string{"config-in-root", "min-version", "naming-conventions", "plugin-sources", "root-dir"}
	newRootCmd("", "", "").PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "config" {
			return
		}
		top, sub, nested := strings.Cut(f.Name, ".")
		if nested {
			assert.Contains(t, schema.Properties[top].Properties, sub, f.Name)
		}
		want = append(want, top)
	})
	assert.ElementsMatch(t, unique(want), keys(schema.Properties))
}

func keys(m map[string]jsonSchema) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func unique(in []string) []string {
	sort.Strings(in)
	out := in[:0]
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}
