package myks

import (
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasYttLogicRe(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"plain values header", "#@data/values\n---\nfoo: bar\n", false},
		{"schema header with overlay", "#@data/values-schema\n#@overlay/match-child-defaults missing_ok=True\n---\nfoo: bar\n", false},
		{"overlay remove", "#@data/values\n---\n#@overlay/remove\nfoo: bar\n", true},
		{"overlay replace", "#@data/values\n---\n#@overlay/replace\nfoo: bar\n", true},
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
		{1.0, "1.0"},                            // integral floats keep the dot to stay floats in KCL
		{"literal ${VAR}", `"literal \${VAR}"`}, // KCL interpolates ${...} in string literals
	}
	for _, tt := range tests {
		got, err := kclScalar(tt.value)
		assert.NoError(t, err)
		assert.Equal(t, tt.want, got)
	}

	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := kclScalar(value)
		assert.Error(t, err, "non-finite floats have no KCL representation")
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

	assign := &kclWriter{}
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

	merge := &kclWriter{}
	writeKclEntries(merge, map[string]any{"nested": map[string]any{"inner": 1}, "scalar": "v"}, 0, true)
	assert.Equal(t, `nested: {
    inner = 1
}
scalar = "v"
`, merge.String())
}

func TestExtractEnvironmentScope(t *testing.T) {
	t.Parallel()
	m := &migrator{}
	node := &migNode{dir: "envs/dev", envValues: map[string]any{
		"helm": map[string]any{"removeLabels": true},
		"environment": map[string]any{
			"id":           "dev",
			"applications": []any{map[string]any{"proto": "webapp", "name": "echo"}, map[string]any{"proto": "argocd"}},
			"baseDomain":   "example.com",
		},
	}}
	m.extractEnvironmentScope(node)

	assert.Equal(t, map[string]any{"baseDomain": "example.com"}, node.envValues["environment"],
		"only id and applications are engine-owned")
	assert.Equal(t, []migApp{{name: "echo", proto: "webapp"}, {name: "argocd", proto: "argocd"}}, node.rawRoster)
	assert.Empty(t, m.warnings)
}

func TestWithoutEngineEnvKeys(t *testing.T) {
	t.Parallel()
	assert.Equal(t,
		map[string]any{"environment": map[string]any{"baseDomain": "example.com"}},
		withoutEngineEnvKeys(map[string]any{"environment": map[string]any{
			"id": "dev", "applications": []any{}, "baseDomain": "example.com",
		}}))
	assert.Equal(t, map[string]any{},
		withoutEngineEnvKeys(map[string]any{"environment": map[string]any{"id": "dev"}}),
		"a scope holding only engine keys is dropped entirely")
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

func TestRefuseExisting(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "env.k")
	assert.NoError(t, refuseExisting([]string{missing}))

	assert.NoError(t, os.WriteFile(missing, []byte("env = {}\n"), 0o600))
	err := refuseExisting([]string{missing})
	assert.ErrorContains(t, err, "refusing to overwrite")
}

// TestWriteProtoK pins the index signature: KCL does not inherit m.App's into a generated
// subclass, so an application declaring a key the prototype's app-data lacks fails to compile
// without it.
func TestWriteProtoK(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := &migrator{
		g:            &Globe{Config: Config{RootDir: dir, PrototypesDir: "prototypes"}},
		protoSchemas: map[string]string{"kb_mcp": "KbMcp"},
		protoBase:    map[string]map[string]any{"kb_mcp": {"helm": map[string]any{"removeLabels": true}}},
	}
	require.NoError(t, m.writeProtoK("kb_mcp"))

	content, err := os.ReadFile(filepath.Join(dir, "prototypes", "kb_mcp", protoKFileName))
	require.NoError(t, err)
	assert.Contains(t, string(content), "schema KbMcp(m.App):\n    [...str]: any\n    proto: str = \"kb_mcp\"\n")
}

func TestSanitizeKclIdentifier(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{
		"cert-manager":               "cert_manager",
		"victoria-metrics-k8s-stack": "victoria_metrics_k8s_stack",
		"already_fine":               "already_fine",
		"dots.and spaces":            "dots_and_spaces",
		"2fa":                        "", // a leading digit cannot be repaired
		"map":                        "", // a KCL builtin type name
		"m":                          "", // shadows the myks import the level files bind
		"parent":                     "", // shadows the parent-level import
		"_apps":                      "", // shadows the per-application accumulator
	} {
		assert.Equal(t, want, sanitizeKclIdentifier(name), name)
	}
}

func TestKclSchemaName(t *testing.T) {
	assert.Equal(t, "Webapp", kclSchemaName("webapp"))
	assert.Equal(t, "PerChartOverride", kclSchemaName("per_chart_override"))
	assert.Equal(t, "Starbase80", kclSchemaName("starbase80"))
}

// TestPlanPrototypeSchemas covers which prototypes get a generated base schema: a prototype
// that does not qualify is not an error, its defaults keep being hoisted into declarations.
func TestPlanPrototypeSchemas(t *testing.T) {
	m := &migrator{
		g: &Globe{Config: Config{PrototypesDir: "prototypes"}},
		protoBase: map[string]map[string]any{
			"webapp":       {"application": map[string]any{"ingress": true}},
			"cert-manager": {"application": map[string]any{"ingress": true}},
			"parent":       {"application": map[string]any{"ingress": true}},
			"no_data":      {},
			"odd_keys":     {"a-b": 1},
			"own_proto":    {"proto": "elsewhere"},
		},
	}
	m.planPrototypeSchemas()
	assert.Equal(t, map[string]string{"webapp": "Webapp"}, m.protoSchemas)
	assert.Len(t, m.warnings, 4, "every skipped prototype with values is reported")
}

// TestRenderLevelFiles pins the split level layout: env.k carries the level's own values and
// folds the accumulator the per-application files unify into, patch.k the frozen environment
// values, and each application file everything the level says about that application.
func TestRenderLevelFiles(t *testing.T) {
	t.Parallel()
	m := &migrator{
		g:            &Globe{Config: Config{PrototypesDir: "prototypes"}},
		nodes:        map[string]*migNode{},
		protoSchemas: map[string]string{},
	}
	m.root = m.newNode("envs", nil)
	leaf := m.newNode(filepath.Join("envs", "dev"), m.root)
	leaf.env = &Environment{ID: "dev"}
	leaf.declared["web"] = migApp{name: "web", proto: "web", values: map[string]any{"replicas": 3}}
	leaf.overrides["cache"] = map[string]any{"replicas": 1}
	leaf.appPatches["web"] = map[string]any{"computed": "x"}
	leaf.envPatch = map[string]any{"computed": "y"}

	rootFiles, err := m.renderNodeFiles(m.root, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{envKFileName}, slices.Sorted(maps.Keys(rootFiles)))

	files, err := m.renderNodeFiles(leaf, m.root)
	require.NoError(t, err)
	assert.Equal(t,
		[]string{"app-cache.k", "app-web.k", envKFileName, patchKFileName},
		slices.Sorted(maps.Keys(files)))

	assert.Contains(t, files[envKFileName], "_apps: m.Apps {}\n")
	assert.Contains(t, files[envKFileName],
		"_lvl = parent.env | {\n    id = \"dev\"\n    applications: {k: v for k, v in _apps}\n}\n")
	assert.Contains(t, files[envKFileName], "env = m.finalize(_lvl | _patch)\n")

	// Declaration and frozen values of one application, in that order: the later block wins.
	assert.Contains(t, files["app-web.k"],
		"_apps: m.Apps {\n    web = m.App {\n        replicas = 3\n    }\n}\n")
	assert.Contains(t, files["app-web.k"], "_apps: m.Apps {\n    web: {\n        computed = \"x\"\n    }\n}\n")
	assert.Contains(t, files["app-cache.k"], "_apps: m.Apps {\n    cache: {\n        replicas = 1\n    }\n}\n")

	assert.Contains(t, files[patchKFileName], "_patch = {\n    computed = \"y\"\n}\n")
}
