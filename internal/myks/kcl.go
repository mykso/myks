package myks

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	version "github.com/hashicorp/go-version"
	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
	kcl "kcl-lang.io/kcl-go"
)

const (
	// kclModFileName is the KCL module manifest; its presence at the config root selects KCL mode.
	kclModFileName = "kcl.mod"
	// protoKFileName holds a prototype's base schema, subclassing myks.App with the
	// prototype's default values; applications instantiate it.
	protoKFileName = "proto.k"
	// Generated ytt bridge files (per environment and per application). Each config unit is
	// split in two because ytt forbids mixing schema and plain data-values documents in one file:
	// the schema part declares the keys the embedded data schema does not, the values part sets
	// the values of the keys it does.
	kclEnvSchemaFileName = "env-data.kcl-schema.ytt.yaml"
	kclEnvValuesFileName = "env-data.kcl-values.ytt.yaml"
	kclAppSchemaFileName = "app-data.kcl-schema.ytt.yaml"
	kclAppValuesFileName = "app-data.kcl-values.ytt.yaml"
)

// supportedKclSchemaVersion is the myks schema version the engine understands.
// Kept in lockstep with kcl/myks/version.k; compatibility is same major.minor.
const supportedKclSchemaVersion = "0.2.0"

// kclTree is the frozen resolved tree produced by evaluating the KCL config root.
// It is the sole discovery mechanism for environments and applications in KCL mode.
type kclTree struct {
	// MyksSchemaVersion is the version of the myks schema the tree was built against.
	MyksSchemaVersion string `yaml:"myksSchemaVersion"`
	// Environments maps environment paths (relative to the environments base dir) to their resolved data.
	Environments map[string]kclEnvironmentData `yaml:"environments"`
}

// kclEnvironmentData is one environment entry of the frozen tree.
type kclEnvironmentData struct {
	// ID is the unique environment (cluster) identifier.
	ID string `yaml:"id"`
	// ArgoCD carries env-level ArgoCD settings.
	ArgoCD map[string]any `yaml:"argocd"`
	// Applications maps application names to their self-contained resolved config.
	Applications map[string]map[string]any `yaml:"applications"`
	// Extra captures the remaining env-level keys (e.g. the myks scope or a global value bag);
	// they flow into the environment data values like legacy env-data content does.
	Extra map[string]any `yaml:",inline"`
}

// isKclMode reports whether the repo opts into the KCL config layer (kcl.mod at the config root).
// A stat error is propagated: silently falling back to legacy mode on a KCL repo would render wrong output.
func (g *Globe) isKclMode() (bool, error) {
	if g.forceLegacyMode {
		return false, nil
	}
	ok, err := isExist(filepath.Join(g.RootDir, kclModFileName))
	if err != nil {
		return false, fmt.Errorf("checking for %s: %w", kclModFileName, err)
	}
	return ok, nil
}

// loadKclTree evaluates the KCL config root once and caches the result on the Globe.
func (g *Globe) loadKclTree() (*kclTree, error) {
	if g.kclTreeCache != nil {
		return g.kclTreeCache, nil
	}
	tree, err := evalKclTree(g.RootDir)
	if err != nil {
		return nil, err
	}
	g.kclTreeCache = tree
	return tree, nil
}

// evalKclTree evaluates the KCL module at rootDir and parses the frozen resolved tree.
func evalKclTree(rootDir string) (*kclTree, error) {
	// The resolver treats a relative manifest path as relative to its own cache dir, not the cwd.
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolving absolute path of %s: %w", rootDir, err)
	}
	deps, err := kcl.UpdateDependencies(&kcl.UpdateDependenciesArgs{ManifestPath: absRootDir})
	if err != nil {
		return nil, fmt.Errorf("resolving KCL dependencies at %s: %w", rootDir, err)
	}
	// KCL hides underscore-prefixed attributes from its output; config values are data, and a
	// leading underscore is a legitimate key (myks repos use `_` for cross-step private values).
	opts := make([]kcl.Option, 0, len(deps.ExternalPkgs)+1)
	opts = append(opts, kcl.WithShowHidden(true))
	for _, pkg := range deps.ExternalPkgs {
		opts = append(opts, kcl.WithExternalPkgAndPath(pkg.PkgName, pkg.PkgPath))
	}

	res, err := kcl.Run(rootDir, opts...)
	if err != nil {
		return nil, fmt.Errorf("evaluating KCL config at %s: %w", rootDir, err)
	}

	tree := &kclTree{}
	if err := yaml.Unmarshal([]byte(res.GetRawYamlResult()), tree); err != nil {
		return nil, fmt.Errorf("parsing KCL evaluation output: %w", err)
	}

	if tree.MyksSchemaVersion == "" {
		return nil, fmt.Errorf("KCL evaluation output is missing myksSchemaVersion")
	}
	if err := checkKclSchemaVersion(tree.MyksSchemaVersion); err != nil {
		return nil, err
	}

	// A nil map means the key is absent (an explicit empty mapping unmarshals to a non-nil map).
	if tree.Environments == nil {
		return nil, fmt.Errorf("KCL evaluation output is missing environments")
	}

	envPathByID := map[string]string{}
	for _, envPath := range slices.Sorted(maps.Keys(tree.Environments)) {
		id := tree.Environments[envPath].ID
		if prev, ok := envPathByID[id]; ok {
			return nil, fmt.Errorf("duplicate environment id %q used by both %s and %s", id, prev, envPath)
		}
		envPathByID[id] = envPath
	}

	return tree, nil
}

// checkKclSchemaVersion asserts the tree's schema version is compatible with the engine.
// Compatible means same major.minor as supportedKclSchemaVersion; patch versions may differ.
func checkKclSchemaVersion(schemaVersion string) error {
	parsed, err := version.NewSemver(schemaVersion)
	core, _, _ := strings.Cut(schemaVersion, "+")
	core, _, _ = strings.Cut(core, "-")
	if err != nil || len(strings.Split(core, ".")) != 3 {
		return fmt.Errorf("malformed myksSchemaVersion %q: expected a semver like %s", schemaVersion, supportedKclSchemaVersion)
	}
	want, err := version.NewSemver(supportedKclSchemaVersion)
	if err != nil {
		return fmt.Errorf("parsing supported KCL schema version %q: %w", supportedKclSchemaVersion, err)
	}
	gotSegments, wantSegments := parsed.Segments(), want.Segments()
	if gotSegments[0] != wantSegments[0] || gotSegments[1] != wantSegments[1] {
		return fmt.Errorf(
			"unsupported myksSchemaVersion %s: this myks build supports %s.x — align the myks schema package pinned in kcl.mod with the myks version",
			schemaVersion, fmt.Sprintf("%d.%d", wantSegments[0], wantSegments[1]))
	}
	return nil
}

// initFromKclTree initializes environments and applications from the frozen tree.
// No filesystem walk is performed: the tree is the sole discovery mechanism.
func (g *Globe) initFromKclTree(envSearchPathToAppMap EnvAppMap) error {
	tree, err := g.loadKclTree()
	if err != nil {
		return err
	}
	log.Debug().Str("myksSchemaVersion", tree.MyksSchemaVersion).Int("environments", len(tree.Environments)).
		Msg(g.Msg("Initialized from KCL frozen tree"))

	filter := g.AddBaseDirToEnvAppMap(envSearchPathToAppMap)
	// ponytail: sequential env init (ignores asyncLevel); parallelize with process() if KCL repos grow many envs
	for _, envPath := range slices.Sorted(maps.Keys(tree.Environments)) {
		dir := filepath.Join(g.EnvironmentBaseDir, filepath.FromSlash(envPath))
		appNames, matched := matchEnvFilter(dir, filter)
		if !matched {
			continue
		}
		env, err := g.initKclEnvironment(dir, tree.Environments[envPath], appNames)
		if err != nil {
			return fmt.Errorf("initializing KCL environment %s: %w", envPath, err)
		}
		g.environments[dir] = env
	}

	return nil
}

// kclEnvIDToPath resolves an environment ID to its path (relative to the environments base dir)
// via the frozen tree. Returns "" if the ID is unknown or the tree cannot be evaluated.
func (g *Globe) kclEnvIDToPath(envID string) string {
	tree, err := g.loadKclTree()
	if err != nil {
		log.Warn().Err(err).Msg(g.Msg("Unable to evaluate KCL tree for environment ID resolution"))
		return ""
	}
	for envPath, envData := range tree.Environments {
		if envData.ID == envID {
			return filepath.FromSlash(envPath)
		}
	}
	return ""
}

// matchEnvFilter checks an environment dir against the CLI env/app selection.
// An empty filter matches everything. A dir matches a filter entry when it equals the
// entry's path or lies below it. Nil app names mean "all applications".
func matchEnvFilter(dir string, filter EnvAppMap) (appNames []string, matched bool) {
	if len(filter) == 0 {
		return nil, true
	}
	all := false
	for searchPath, names := range filter {
		if dir != searchPath && !strings.HasPrefix(dir, searchPath+string(filepath.Separator)) {
			continue
		}
		matched = true
		if len(names) == 0 {
			all = true
		}
		appNames = append(appNames, names...)
	}
	if !matched || all {
		return nil, matched
	}
	return unique(appNames), true
}

// initKclEnvironment builds a fully initialized Environment from one frozen-tree entry.
// The tree entry is materialized as generated ytt bridge files (one env-level pair and one pair
// per application), after which the regular environment initialization runs on top of them, so
// legacy semantics (embedded schema defaults, env data lib, ArgoCD settings) apply unchanged.
func (g *Globe) initKclEnvironment(dir string, envData kclEnvironmentData, appNames []string) (*Environment, error) {
	if envData.ID == "" {
		return nil, fmt.Errorf("environment entry is missing id")
	}

	serviceDir := filepath.Join(g.RootDir, g.ServiceDirName, dir)
	env := &Environment{
		Dir:                     dir,
		ID:                      envData.ID,
		Applications:            []*Application{},
		g:                       g,
		cfg:                     &g.Config,
		extraYttPaths:           g.extraYttPaths,
		renderedDataLibFilePath: filepath.Join(serviceDir, g.RenderedEnvironmentDataLibFileName),
		foundApplications:       map[string]string{},
		kclMode:                 true,
		kclDataFiles: []string{
			filepath.Join(serviceDir, kclEnvSchemaFileName),
			filepath.Join(serviceDir, kclEnvValuesFileName),
		},
	}

	if err := writeKclDataFiles(env.kclDataFiles[0], env.kclDataFiles[1], envData.dataValues()); err != nil {
		return nil, fmt.Errorf("writing generated environment data values: %w", err)
	}

	for _, name := range slices.Sorted(maps.Keys(envData.Applications)) {
		values := maps.Clone(envData.Applications[name])
		delete(values, "proto")
		appDir := filepath.Join(serviceDir, g.AppsDir, name)
		err := writeKclDataFiles(
			filepath.Join(appDir, kclAppSchemaFileName),
			filepath.Join(appDir, kclAppValuesFileName),
			values,
		)
		if err != nil {
			return nil, fmt.Errorf("writing generated data values for app %s: %w", name, err)
		}
	}

	if err := env.Init(appNames); err != nil {
		return nil, err
	}

	return env, nil
}

// dataValues shapes the environment entry as ytt data values: extras stay top-level scopes,
// id and the application roster go under the environment scope. The engine owns only those
// two keys there; an `environment` extra carrying further keys is merged underneath them,
// so a level can publish shared values under the scope its templates already read.
func (d kclEnvironmentData) dataValues() map[string]any {
	values := map[string]any{}
	maps.Copy(values, d.Extra)
	if d.ArgoCD != nil {
		values["argocd"] = d.ArgoCD
	}

	apps := make([]map[string]any, 0, len(d.Applications))
	for _, name := range slices.Sorted(maps.Keys(d.Applications)) {
		proto, _ := d.Applications[name]["proto"].(string)
		if proto == "" {
			proto = name
		}
		apps = append(apps, map[string]any{"name": name, "proto": proto})
	}
	envScope := map[string]any{}
	if extra, ok := values["environment"].(map[string]any); ok {
		maps.Copy(envScope, extra)
	}
	envScope["id"] = d.ID
	envScope["applications"] = apps
	values["environment"] = envScope

	return values
}

// declaredDataSchema is the embedded data schema parsed as plain YAML. ytt's `#@`/`#!`
// annotations are YAML comments, so the parse yields exactly the set of declared keys.
var declaredDataSchema = sync.OnceValues(func() (map[string]any, error) {
	declared := map[string]any{}
	if err := yaml.Unmarshal(dataSchema, &declared); err != nil {
		return nil, fmt.Errorf("parsing embedded data schema: %w", err)
	}
	return declared, nil
})

// writeKclDataFiles writes one resolved config unit as a pair of generated ytt files:
// keys unknown to the embedded data schema become a schema-extension document, keys it
// declares become a plain data-values document. Both files are always written (with an empty
// body when there is nothing to say) so no stale content survives a re-run.
func writeKclDataFiles(schemaPath, valuesPath string, values map[string]any) error {
	schema, err := declaredDataSchema()
	if err != nil {
		return err
	}
	declared, unknown := splitDeclared(values, schema)

	body := &strings.Builder{}
	if err := renderSchemaExtension(body, unknown, schema, 0); err != nil {
		return err
	}
	if body.Len() == 0 {
		body.WriteString("{}\n")
	}
	header := "#@data/values-schema\n#@overlay/match-child-defaults missing_ok=True\n---\n"
	if err := writeFile(schemaPath, []byte(header+body.String())); err != nil {
		return err
	}

	yamlBytes, err := yaml.Marshal(declared)
	if err != nil {
		return fmt.Errorf("marshalling generated data values: %w", err)
	}
	return writeFile(valuesPath, append([]byte("#@data/values\n---\n"), yamlBytes...))
}

// splitDeclared splits resolved values against the shape of the embedded data schema. Only
// maps are walked in parallel: a key the schema does not declare is unknown together with
// everything below it, anything else is declared.
func splitDeclared(values, schema map[string]any) (declared, unknown map[string]any) {
	declared, unknown = map[string]any{}, map[string]any{}
	for key, value := range values {
		sub, ok := schema[key]
		if !ok {
			unknown[key] = value
			continue
		}
		subSchema, schemaIsMap := sub.(map[string]any)
		valueMap, valueIsMap := value.(map[string]any)
		if !schemaIsMap || !valueIsMap || len(valueMap) == 0 {
			declared[key] = value
			continue
		}
		subDeclared, subUnknown := splitDeclared(valueMap, subSchema)
		if len(subDeclared) > 0 {
			declared[key] = subDeclared
		}
		if len(subUnknown) > 0 {
			unknown[key] = subUnknown
		}
	}
	return declared, unknown
}

// renderSchemaExtension renders the body of the schema-extension document. Keys the embedded
// schema declares are walked through as parents so that undeclared keys nest inside their
// declared scope; every undeclared key is emitted as its own schema definition. A definition
// holding an array anywhere is typed `any`, because a ytt schema array declares only the type
// of its single item and always defaults to an empty list, which would drop the value.
func renderSchemaExtension(b *strings.Builder, values, schema map[string]any, indent int) error {
	pad := strings.Repeat(" ", indent)
	for _, key := range slices.Sorted(maps.Keys(values)) {
		value := values[key]
		subSchema, isDeclaredScope := schema[key].(map[string]any)
		valueMap, isMap := value.(map[string]any)
		if isDeclaredScope && isMap {
			head, err := marshalMapItem(key, nil)
			if err != nil {
				return err
			}
			// The placeholder value is dropped so the children nest under the bare key.
			b.WriteString(pad + strings.TrimSuffix(head, " null") + "\n")
			if err := renderSchemaExtension(b, valueMap, subSchema, indent+2); err != nil {
				return err
			}
			continue
		}
		if containsArray(value) {
			b.WriteString(pad + "#@schema/type any=True\n")
		}
		item, err := marshalMapItem(key, value)
		if err != nil {
			return err
		}
		for line := range strings.SplitSeq(item, "\n") {
			b.WriteString(pad + line + "\n")
		}
	}
	return nil
}

// marshalMapItem renders a single map item as YAML (without trailing newline), leaving key
// quoting to the YAML marshaller.
func marshalMapItem(key string, value any) (string, error) {
	yamlBytes, err := yaml.Marshal(map[string]any{key: value})
	if err != nil {
		return "", fmt.Errorf("marshalling generated data values: %w", err)
	}
	return strings.TrimRight(string(yamlBytes), "\n"), nil
}

func containsArray(value any) bool {
	switch typed := value.(type) {
	case []any:
		return true
	case map[string]any:
		for _, nested := range typed {
			if containsArray(nested) {
				return true
			}
		}
	}
	return false
}
