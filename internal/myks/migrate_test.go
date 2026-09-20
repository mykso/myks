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

func TestFileComputes(t *testing.T) {
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
		{"schema default annotation", "#@data/values-schema\n---\n#@schema/default [\"x\"]\nitems: ['']\n", false},
		{"schema nullable annotation", "#@data/values-schema\n---\n#@schema/nullable\nfoo: ''\n", false},
		{"schema validation annotation", "#@data/values-schema\n---\n#@schema/validation min_len=1\nimage: ''\n", false},
		{"schema type annotation", "#@data/values-schema\n---\n#@schema/type any=True\nfoo: bar\n", false},
		{"schema doc with a load directive", "#@ load(\"@ytt:data\", \"data\")\n#@data/values-schema\n---\nfoo: bar\n", true},
		{"plain comment", "#! just a comment\n#@data/values\n---\nfoo: bar\n", false},
		{"templated string", "#@data/values\n---\n#@yaml/text-templated-strings\nfoo: |\n  (@= x @)\n", true},
		{"overlay match", "#@data/values\n---\n#@overlay/match by=\"name\"\nfoo: bar\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, fileComputes([]byte(tt.content)))
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
	writeKclEntries(assign, values, 0, false, "")
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
	writeKclEntries(merge, map[string]any{"nested": map[string]any{"inner": 1}, "scalar": "v"}, 0, true, "")
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
// TestConvertDataFile covers how a data file is read: a schema document through ytt (so the
// schema semantics plain YAML cannot see are applied), a plain document as YAML, and a
// computed one not at all.
func TestConvertDataFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := &migrator{g: &Globe{Config: Config{RootDir: dir}}}
	write := func(t *testing.T, name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}

	t.Run("schema document is resolved by ytt", func(t *testing.T) {
		path := write(t, "schema.ytt.yaml", `#@data/values-schema
---
application:
  #@schema/validation min_len=1
  image: ''
  #! a schema array carries only the type of its item and defaults to empty
  env:
  - name: TZ
  #@schema/default 2
  replicas: 1
`)
		converted, err := m.convertDataFile(path)
		require.NoError(t, err)
		require.NotNil(t, converted)
		app, ok := converted.values["application"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "", app["image"])
		assert.Empty(t, app["env"], "a schema array defaults to empty, whatever its item says")
		assert.Equal(t, 2, app["replicas"], "#@schema/default wins over the written value")
		require.NotNil(t, converted.schema)
		assert.Equal(t, "{str:any}", kclType(converted.schema.nodeAt([]string{"application"})))
		require.Len(t, converted.schema.constraints, 1)
		assert.Equal(t, []string{"application", "image"}, converted.schema.constraints[0].path)
	})

	t.Run("plain document is parsed as YAML", func(t *testing.T) {
		path := write(t, "values.ytt.yaml", "#@data/values\n---\napplication:\n  env:\n  - name: TZ\n")
		converted, err := m.convertDataFile(path)
		require.NoError(t, err)
		require.NotNil(t, converted)
		app := converted.values["application"].(map[string]any)
		assert.Len(t, app["env"], 1, "a plain data-values list is a value, not a schema")
		assert.Nil(t, converted.schema)
	})

	t.Run("computed values are split out, the plain ones are kept", func(t *testing.T) {
		path := write(t, "computed.ytt.yaml", `#@ load("@myks:data.lib.yaml", "env_data")

#@data/values-schema
---
application:
  baseValue: true
  envId: #@ env_data.environment.id
`)
		converted, err := m.convertDataFile(path)
		require.NoError(t, err)
		require.NotNil(t, converted)
		app := converted.values["application"].(map[string]any)
		assert.Equal(t, map[string]any{"baseValue": true}, app, "only the computed value is left to the leaf patch")
		assert.Contains(t, m.skipped, skippedFile{file: path, deferred: []string{".application.envId"}})
	})

	t.Run("a document that cannot be split is skipped whole", func(t *testing.T) {
		path := write(t, "all-computed.ytt.yaml", "#@ load(\"@myks:data.lib.yaml\", \"env_data\")\n#@data/values-schema\n---\nfoo: #@ env_data.environment.id\n")
		converted, err := m.convertDataFile(path)
		require.NoError(t, err)
		assert.Nil(t, converted)
		assert.Contains(t, m.skipped, skippedFile{file: path})
	})
}

// TestCarryValidations verifies that `not_null` reaches the schema as a constraint and that
// the validations no KCL expression can state are reported, by path, instead of silently
// dropped.
func TestCarryValidations(t *testing.T) {
	t.Parallel()
	m := &migrator{}
	schema := &inspectedSchema{}
	m.carryValidations("app-data.ytt.yaml", []byte(`#@data/values-schema
---
#@schema/validation min_len=1, not_null=True
a: ''
#@schema/validation ("must be lowercase", lambda v: v == v.lower())
b: ''
application:
  #@schema/validation one_of=["x"]
  c: 'x'
  #@schema/validation when=lambda v: True, min_len=1
  d: 'y'
`), schema)
	assert.Equal(t, []schemaConstraint{{path: []string{"a"}, kind: constraintNotNull}}, schema.constraints)
	require.Len(t, m.warnings, 1)
	assert.Contains(t, m.warnings[0], "b (custom rule)")
	assert.Contains(t, m.warnings[0], "application.d (when)")
	assert.NotContains(t, m.warnings[0], "min_len")
	assert.NotContains(t, m.warnings[0], "application.c")
}

func TestWriteProtoK(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	schema, err := parseSchemaInspect([]byte(`
components:
  schemas:
    dataValues:
      type: object
      properties:
        image: {type: string, default: "kb-mcp:1.0.0", minLength: 1}
`))
	require.NoError(t, err)
	values := map[string]any{
		"helm":  map[string]any{"removeLabels": true},
		"image": "kb-mcp:1.0.0",
	}
	m := &migrator{
		g:              &Globe{Config: Config{RootDir: dir, PrototypesDir: "prototypes"}},
		protoSchemas:   map[string]string{"kb_mcp": "KbMcp"},
		protoBase:      map[string]map[string]any{"kb_mcp": values},
		protoInspected: map[string]*inspectedSchema{"kb_mcp": schema},
		protoPlans:     map[string]*protoSchemaPlan{"kb_mcp": newProtoSchemaPlan("KbMcp", values, schema)},
	}
	require.NoError(t, m.writeProtoK("kb_mcp"))

	content, err := os.ReadFile(filepath.Join(dir, "prototypes", "kb_mcp", protoKFileName))
	require.NoError(t, err)
	assert.Contains(t, string(content), "schema KbMcp(m.App):\n    [...str]: any\n    proto: str = \"kb_mcp\"\n")
	assert.Contains(t, string(content), "image?: str = \"kb-mcp:1.0.0\"", "the inspected schema types the attribute")
	assert.Contains(t, string(content), "\n    check:\n        len(image) >= 1, \"image must be at least 1 long\"\n")
	assert.Contains(t, string(content), "helm?: {str:any} = {", "a value the schema does not describe stays a literal")
}

// TestWriteProtoKNested pins the two things a structured object value buys: its fields stay
// visible and typed in a schema of their own, and a validation the prototype's default
// violates keeps the field — declared without a default, its check guarded against the
// absence, so it fires only once a level sets the value.
func TestWriteProtoKNested(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	schema, err := parseSchemaInspect([]byte(`
components:
  schemas:
    dataValues:
      type: object
      properties:
        image: {type: string, default: "", minLength: 1}
        application:
          type: object
          properties:
            name: {type: string, default: "", minLength: 1}
            containerPort: {type: integer, default: 80}
            ingress: {type: boolean, default: true}
`))
	require.NoError(t, err)
	values := schema.defaults
	pruneDemandedDefaults(values, schema)
	assert.Equal(t, map[string]bool{"image": true, pathKey([]string{"application", "name"}): true}, schema.demanded)

	m := &migrator{
		g:              &Globe{Config: Config{RootDir: dir, PrototypesDir: "prototypes"}},
		protoSchemas:   map[string]string{"webapp": "Webapp"},
		protoBase:      map[string]map[string]any{"webapp": values},
		protoInspected: map[string]*inspectedSchema{"webapp": schema},
		protoPlans:     map[string]*protoSchemaPlan{"webapp": newProtoSchemaPlan("Webapp", values, schema)},
	}
	require.NoError(t, m.writeProtoK("webapp"))

	content, err := os.ReadFile(filepath.Join(dir, "prototypes", "webapp", protoKFileName))
	require.NoError(t, err)
	assert.Contains(t, string(content), "    application?: WebappApplication = WebappApplication {}\n")
	assert.Contains(t, string(content), "    image?: str\n", "a demanded value is declared without a default")
	assert.Contains(t, string(content),
		"schema WebappApplication:\n    [...str]: any\n    containerPort?: int = 80\n    ingress?: bool = True\n    name?: str\n")
	assert.Contains(t, string(content), `len(name) >= 1 if name != Undefined, "application.name must be at least 1 long"`,
		"a nested check lives in the schema that owns the field")
	assert.Contains(t, string(content), `len(image) >= 1 if image != Undefined, "image must be at least 1 long"`)
	assert.Empty(t, m.warnings)
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

// TestWriteProtoKArrayElements pins what an array whose ytt schema describes its element
// buys: the element becomes a schema of its own, so KCL fills in the fields an application's
// array leaves out — what ytt did by overlaying that array onto the schema's. It also pins
// `not_null`, which ytt's OpenAPI output drops and the converter reads from the annotation.
func TestWriteProtoKArrayElements(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := []byte(`#@data/values-schema
---
application:
  clients:
    - host: ""
      port: 5001
      insecureSkipVerify: false
  #@schema/type any=True
  #@schema/nullable
  #@schema/validation not_null=True
  registries:
`)
	schema, err := parseSchemaInspect([]byte(`
components:
  schemas:
    dataValues:
      type: object
      properties:
        application:
          type: object
          properties:
            clients:
              type: array
              default: []
              items:
                type: object
                properties:
                  host: {type: string, default: ""}
                  port: {type: integer, default: 5001}
                  insecureSkipVerify: {type: boolean, default: false}
            registries: {type: "null", nullable: true, default: null}
`))
	require.NoError(t, err)

	m := &migrator{g: &Globe{Config: Config{RootDir: dir, PrototypesDir: "prototypes"}}}
	m.carryValidations("app-data.ytt.yaml", source, schema)
	values := schema.defaults
	pruneDemandedDefaults(values, schema)
	m.protoSchemas = map[string]string{"csi": "Csi"}
	m.protoBase = map[string]map[string]any{"csi": values}
	m.protoInspected = map[string]*inspectedSchema{"csi": schema}
	m.protoPlans = map[string]*protoSchemaPlan{"csi": newProtoSchemaPlan("Csi", values, schema)}
	require.NoError(t, m.writeProtoK("csi"))

	content, err := os.ReadFile(filepath.Join(dir, "prototypes", "csi", protoKFileName))
	require.NoError(t, err)
	assert.Contains(t, string(content), "    clients?: [CsiApplicationClients] = []\n")
	assert.Contains(t, string(content),
		"schema CsiApplicationClients:\n    [...str]: any\n    host?: str = \"\"\n    insecureSkipVerify?: bool = False\n    port?: int = 5001\n")
	assert.Contains(t, string(content), "    registries?: any\n", "not_null prunes the null default")
	assert.Contains(t, string(content),
		`registries != None if registries != Undefined, "application.registries must not be null"`)
	assert.Empty(t, m.warnings)

	// The element defaults the patch simulation expects are the ones KCL will fill in.
	completed := m.protoPlans["csi"].withElementDefaults(map[string]any{
		"application": map[string]any{"clients": []any{map[string]any{"host": "h"}}},
	}, nil)
	assert.Equal(t, []any{map[string]any{"host": "h", "port": 5001, "insecureSkipVerify": false}},
		completed["application"].(map[string]any)["clients"])
}
