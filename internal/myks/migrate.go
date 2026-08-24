package myks

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

// Migrate converts a legacy ytt data-values repo into a KCL config tree (roadmap step 4).
//
// The converter machine-seeds the tree: plain-YAML data-values files are converted in place
// on the level/import structure, the application roster is generated from the resolved
// environment data, and values produced by ytt logic (which cannot be translated) are frozen
// as literals in a leaf-level patch marked with TODO comments. The byte-identical gate
// (render both modes, diff rendered/) validates the result; hand-finishing is guided by
// docs/migration.md.
//
// schemaPackage selects the myks KCL schema package dependency: an oci:// reference or a
// local path.
func Migrate(g *Globe, schemaPackage string) error {
	if kclMode, err := g.isKclMode(); err != nil {
		return err
	} else if kclMode {
		return fmt.Errorf("%s already exists: this repo is already in KCL mode", kclModFileName)
	}
	if err := g.ValidateRootDir(); err != nil {
		return err
	}
	if err := g.Init(1, nil); err != nil {
		return fmt.Errorf("initializing legacy repo: %w", err)
	}
	if len(g.environments) == 0 {
		return errors.New("no environments found, nothing to migrate")
	}

	m := &migrator{g: g, nodes: map[string]*migNode{}, protoBase: map[string]map[string]any{}}
	if err := m.buildTree(); err != nil {
		return err
	}
	if err := m.collectContributions(); err != nil {
		return err
	}
	m.placeApplications()
	if err := m.computePatches(); err != nil {
		return err
	}
	if err := m.emit(schemaPackage); err != nil {
		return err
	}
	m.printReport()
	return nil
}

// migrator carries the intermediate state of one conversion run.
type migrator struct {
	g *Globe
	// nodes maps node dirs (e.g. "envs", "envs/dev") to their conversion state.
	nodes map[string]*migNode
	root  *migNode
	// protoBase holds the converted prototypes/<proto>/app-data values (root-level contributions).
	protoBase map[string]map[string]any
	// skipped lists data files containing ytt logic; their values are frozen into leaf patches.
	skipped []string
	// warnings lists conditions the user must resolve by hand.
	warnings []string
	// patched counts leaf-level patched value paths.
	patched int
}

// migNode is one level of the environment tree under conversion.
type migNode struct {
	dir    string // relative to the repo root, e.g. "envs" or "envs/dev"
	parent *migNode
	env    *Environment // set for leaves
	// leaves lists the environments at or below this node.
	leaves []*migNode
	// envValues are the raw-converted env-data values of this level (environment scope stripped).
	envValues map[string]any
	// envPatch holds leaf-level env values frozen from the legacy-resolved output.
	envPatch map[string]any
	// rawRoster lists the roster entries of this level's env-data files, as written.
	rawRoster []migApp
	// declared maps application names to their placed declaration (roster entry with values).
	declared map[string]migApp
	// overrides holds per-application values placed at this level for apps declared above it.
	overrides map[string]map[string]any
	// appPatches holds leaf-level per-application values frozen from the legacy-resolved output.
	appPatches map[string]map[string]any
	// protoValues and appValues are the raw _proto/ and _apps/ contributions of this level.
	protoValues map[string]map[string]any
	appValues   map[string]map[string]any
}

type migApp struct {
	name, proto string
	values      map[string]any
}

func (n *migNode) chain() []*migNode {
	var nodes []*migNode
	for cur := n; cur != nil; cur = cur.parent {
		nodes = append(nodes, cur)
	}
	slices.Reverse(nodes)
	return nodes
}

// kclIdentifierRe matches names usable as KCL identifiers (package path components, bare keys).
var kclIdentifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// kclReservedWords are KCL keywords and builtin type names that cannot be bare identifiers.
var kclReservedWords = map[string]bool{
	"True": true, "False": true, "None": true, "Undefined": true,
	"import": true, "and": true, "or": true, "in": true, "is": true, "not": true,
	"as": true, "if": true, "else": true, "elif": true, "for": true,
	"schema": true, "mixin": true, "protocol": true, "check": true, "assert": true,
	"all": true, "any": true, "map": true, "filter": true, "lambda": true, "rule": true,
	"str": true, "int": true, "float": true, "bool": true, "type": true,
}

func isKclIdentifier(s string) bool {
	return kclIdentifierRe.MatchString(s) && !kclReservedWords[s]
}

// buildTree creates a migNode for every directory on the path from the environments base
// dir to each discovered environment. Directory names become KCL package path components,
// so each must be a valid KCL identifier.
func (m *migrator) buildTree() error {
	base := m.g.EnvironmentBaseDir
	if !isKclIdentifier(filepath.Base(base)) {
		return fmt.Errorf("environments base dir %q is not a valid KCL identifier; rename it before migrating", base)
	}
	m.root = m.newNode(base, nil)

	var badDirs []string
	for _, leafDir := range slices.Sorted(maps.Keys(m.g.environments)) {
		if leafDir == base {
			return fmt.Errorf("environment %s is the environments base dir itself; nest it one level deeper before migrating", leafDir)
		}
		rel, err := filepath.Rel(base, leafDir)
		if err != nil {
			return fmt.Errorf("resolving %s against %s: %w", leafDir, base, err)
		}
		parent := m.root
		for component := range strings.SplitSeq(filepath.ToSlash(rel), "/") {
			if !isKclIdentifier(component) {
				badDirs = append(badDirs, filepath.Join(parent.dir, component))
			}
			dir := filepath.Join(parent.dir, component)
			node, ok := m.nodes[dir]
			if !ok {
				node = m.newNode(dir, parent)
			}
			parent = node
		}
		parent.env = m.g.environments[leafDir]
		for _, node := range parent.chain() {
			node.leaves = append(node.leaves, parent)
		}
	}
	if len(badDirs) > 0 {
		return fmt.Errorf(
			"environment directories must be valid KCL identifiers (letters, digits, underscores); rename before migrating: %s",
			strings.Join(unique(badDirs), ", "))
	}
	return nil
}

func (m *migrator) newNode(dir string, parent *migNode) *migNode {
	node := &migNode{
		dir:         dir,
		parent:      parent,
		envValues:   map[string]any{},
		declared:    map[string]migApp{},
		overrides:   map[string]map[string]any{},
		appPatches:  map[string]map[string]any{},
		protoValues: map[string]map[string]any{},
		appValues:   map[string]map[string]any{},
	}
	m.nodes[dir] = node
	return node
}

// collectContributions raw-converts the data-values files of every node (and, at the root,
// of prototypes/). Files containing ytt logic are recorded and skipped: their effect is
// frozen into leaf patches later.
func (m *migrator) collectContributions() error {
	cfg := &m.g.Config
	for _, dir := range slices.Sorted(maps.Keys(m.nodes)) {
		node := m.nodes[dir]
		envValues, err := m.convertFileGlob(filepath.Join(node.dir, cfg.EnvironmentDataFileName))
		if err != nil {
			return err
		}
		node.envValues = envValues
		m.extractEnvironmentScope(node)

		if node.protoValues, err = m.convertPerDirGlobs(filepath.Join(node.dir, cfg.PrototypeOverrideDir), cfg.ApplicationDataFileName); err != nil {
			return err
		}
		if node.appValues, err = m.convertPerDirGlobs(filepath.Join(node.dir, cfg.AppsDir), cfg.ApplicationDataFileName); err != nil {
			return err
		}
	}

	var err error
	if m.protoBase, err = m.convertPerDirGlobs(filepath.Join(m.g.RootDir, cfg.PrototypesDir), cfg.ApplicationDataFileName); err != nil {
		return err
	}
	return nil
}

// extractEnvironmentScope removes the engine-owned environment scope from a node's env
// values: the roster feeds application placement, the id comes from the discovered
// environment, and any other key cannot be represented (the engine regenerates the scope).
func (m *migrator) extractEnvironmentScope(node *migNode) {
	envScope, ok := node.envValues["environment"].(map[string]any)
	if !ok {
		return
	}
	delete(node.envValues, "environment")

	for key, value := range envScope {
		switch key {
		case "id":
			// The leaf id is taken from the discovered environment.
		case "applications":
			entries, _ := value.([]any)
			for _, raw := range entries {
				entry, _ := raw.(map[string]any)
				proto, _ := entry["proto"].(string)
				name, _ := entry["name"].(string)
				if proto == "" {
					// Mirrors the legacy engine, which skips roster entries without a prototype.
					m.warn("%s: roster entry without proto skipped (name: %q)", node.dir, name)
					continue
				}
				if name == "" {
					name = proto
				}
				node.rawRoster = append(node.rawRoster, migApp{name: name, proto: proto})
			}
		default:
			m.warn("%s: environment.%s cannot be migrated (the engine owns the environment scope); move it elsewhere by hand", node.dir, key)
		}
	}
}

// convertFileGlob raw-converts all files matching the glob into one merged value map.
func (m *migrator) convertFileGlob(pattern string) (map[string]any, error) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing %s: %w", pattern, err)
	}
	values := map[string]any{}
	for _, file := range files {
		fileValues, converted, err := m.convertDataFile(file)
		if err != nil {
			return nil, err
		}
		if converted {
			values = mergeValues(values, fileValues)
		}
	}
	return values, nil
}

// convertPerDirGlobs raw-converts <base>/<name>/<filePattern> for every subdirectory of base,
// returning a map keyed by subdirectory name.
func (m *migrator) convertPerDirGlobs(base, filePattern string) (map[string]map[string]any, error) {
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]map[string]any{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", base, err)
	}
	result := map[string]map[string]any{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		values, err := m.convertFileGlob(filepath.Join(base, entry.Name(), filePattern))
		if err != nil {
			return nil, err
		}
		if len(values) > 0 {
			result[entry.Name()] = values
		}
	}
	return result, nil
}

// hasYttLogicRe detects ytt computation in a data file: a directive with code after it
// (`#@ load(...)`, `key: #@ expr`) or a schema annotation that changes values
// (`#@schema/default`). Plain document headers (`#@data/values`, `#@overlay/...`) carry no
// space after `#@` and do not match.
var hasYttLogicRe = regexp.MustCompile(`#@[ \t]|#@schema/`)

// convertDataFile parses one data-values file as plain YAML. Files containing ytt logic
// are skipped (converted=false) and recorded: raw parsing would misread computed values.
func (m *migrator) convertDataFile(file string) (values map[string]any, converted bool, err error) {
	content, err := os.ReadFile(file) // #nosec G304 -- paths come from globbing the repo being migrated
	if err != nil {
		return nil, false, fmt.Errorf("reading %s: %w", file, err)
	}
	if hasYttLogicRe.Match(content) {
		m.skipped = append(m.skipped, file)
		return nil, false, nil
	}

	values = map[string]any{}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	for {
		var doc map[string]any
		if err := decoder.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, false, fmt.Errorf("parsing %s: %w", file, err)
		}
		values = mergeValues(values, doc)
	}
	return values, true, nil
}

// placeApplications turns raw contributions into per-node declarations and overrides.
//
// The resolved per-environment roster (foundApplications) is the truth: an application is
// declared at the highest raw-roster node whose whole subtree contains it (a lower-level
// legacy roster replaces the inherited list, and a dict union cannot remove entries — so a
// declaration must never leak into an environment that dropped the application). Otherwise
// it is declared at the leaf.
//
// A declaration at node D merges (in legacy file order) the prototype app-data, the _proto/
// contributions of D and its ancestors, and the _apps/ contributions of D and its
// ancestors. Nodes below D contribute overrides. The legacy order interleaves _proto and
// _apps across levels differently; any resulting value difference is corrected by the leaf
// patches.
func (m *migrator) placeApplications() {
	for _, leafDir := range slices.Sorted(maps.Keys(m.g.environments)) {
		leaf := m.nodes[leafDir]
		chain := leaf.chain()

		for _, name := range slices.Sorted(maps.Keys(leaf.env.foundApplications)) {
			proto := leaf.env.foundApplications[name]
			decl := leaf
			for _, node := range chain {
				if rawRosterHas(node, name, proto) && allLeavesRun(node, name, proto) {
					decl = node
					break
				}
			}

			if _, ok := decl.declared[name]; !ok {
				values := m.protoBase[proto]
				for _, node := range decl.chain() {
					values = mergeValues(values, node.protoValues[proto])
				}
				for _, node := range decl.chain() {
					values = mergeValues(values, node.appValues[name])
				}
				decl.declared[name] = migApp{name: name, proto: proto, values: values}
			}

			afterDecl := false
			for _, node := range chain {
				if !afterDecl {
					afterDecl = node == decl
					continue
				}
				override := mergeValues(node.protoValues[proto], node.appValues[name])
				if len(override) > 0 {
					node.overrides[name] = override
				}
			}
		}
	}

	for _, dir := range slices.Sorted(maps.Keys(m.nodes)) {
		node := m.nodes[dir]
		for _, name := range slices.Sorted(maps.Keys(node.appValues)) {
			used := slices.ContainsFunc(node.leaves, func(leaf *migNode) bool {
				_, ok := leaf.env.foundApplications[name]
				return ok
			})
			if !used {
				m.warn("%s: %s/%s has data values but no matching application in any environment below", dir, m.g.AppsDir, name)
			}
		}
	}
}

func rawRosterHas(node *migNode, name, proto string) bool {
	return slices.ContainsFunc(node.rawRoster, func(app migApp) bool {
		return app.name == name && app.proto == proto
	})
}

func allLeavesRun(node *migNode, name, proto string) bool {
	for _, leaf := range node.leaves {
		if leaf.env.foundApplications[name] != proto {
			return false
		}
	}
	return true
}

// computePatches freezes, per leaf, every value the raw conversion could not reproduce.
//
// The simulation runs the real engine seam: the tree levels are merged with KCL union
// semantics (mergeValues) and the result is materialized through writeKclDataFiles and
// resolved by ytt — exactly what the engine does with the frozen tree at render time. The
// simulated result is diffed against the legacy-resolved output; differences become
// leaf-level literals marked for hand-finish.
func (m *migrator) computePatches() error {
	for _, leafDir := range slices.Sorted(maps.Keys(m.g.environments)) {
		leaf := m.nodes[leafDir]
		chain := leaf.chain()

		treeEnv := map[string]any{}
		for _, node := range chain {
			treeEnv = mergeValues(treeEnv, node.envValues)
		}
		legacyEnvYaml, err := leaf.env.renderEnvData(leaf.env.envDataFiles())
		if err != nil {
			return fmt.Errorf("resolving legacy env data of %s: %w", leafDir, err)
		}
		legacyEnv := map[string]any{}
		if err := yaml.Unmarshal(legacyEnvYaml, &legacyEnv); err != nil {
			return fmt.Errorf("parsing legacy env data of %s: %w", leafDir, err)
		}
		m.checkEnvironmentScope(leafDir, legacyEnv)

		simEnv, err := m.simulateBridge(leafDir, "env", treeEnv, nil)
		if err != nil {
			return err
		}
		leaf.envPatch = m.diffValues(leafDir, simEnv, legacyEnv)
		treeEnv = mergeValues(treeEnv, leaf.envPatch)
		envBridgeFiles, err := m.writeBridgeFiles(leafDir, "env", treeEnv)
		if err != nil {
			return err
		}

		for _, app := range leaf.env.Applications {
			treeApp := map[string]any{}
			afterDecl := false
			for _, node := range chain {
				if !afterDecl {
					if decl, ok := node.declared[app.Name]; ok {
						treeApp = mergeValues(treeApp, decl.values)
						afterDecl = true
					}
					continue
				}
				treeApp = mergeValues(treeApp, node.overrides[app.Name])
			}

			legacyApp, err := m.inspectDataValues(app.yttDataFiles)
			if err != nil {
				return fmt.Errorf("resolving legacy data values of %s in %s: %w", app.Name, leafDir, err)
			}
			simApp, err := m.simulateBridge(leafDir, "app-"+app.Name, treeApp, envBridgeFiles)
			if err != nil {
				return err
			}
			patch := m.diffValues(leafDir+"/"+app.Name, simApp, legacyApp)
			if len(patch) > 0 {
				leaf.appPatches[app.Name] = patch
			}
		}
	}
	return nil
}

// writeBridgeFiles materializes one config unit exactly like the engine's KCL bridge
// (a schema-extension file and a plain values file) under the service tmp dir.
func (m *migrator) writeBridgeFiles(leafDir, unit string, values map[string]any) ([]string, error) {
	dir := filepath.Join(m.g.RootDir, m.g.ServiceDirName, m.g.TempDirName, "migrate", leafDir)
	files := []string{
		filepath.Join(dir, unit+".kcl-schema.ytt.yaml"),
		filepath.Join(dir, unit+".kcl-values.ytt.yaml"),
	}
	if err := writeKclDataFiles(files[0], files[1], values); err != nil {
		return nil, fmt.Errorf("writing simulated bridge files for %s in %s: %w", unit, leafDir, err)
	}
	return files, nil
}

// simulateBridge resolves one config unit the way the engine will after migration:
// generated bridge files (preceded by the env-level ones, if given) rendered by ytt over
// the embedded schema.
func (m *migrator) simulateBridge(leafDir, unit string, values map[string]any, envBridgeFiles []string) (map[string]any, error) {
	files, err := m.writeBridgeFiles(leafDir, unit, values)
	if err != nil {
		return nil, err
	}
	resolved, err := m.inspectDataValues(concatenate(envBridgeFiles, files))
	if err != nil {
		return nil, fmt.Errorf("resolving simulated bridge values for %s in %s: %w", unit, leafDir, err)
	}
	return resolved, nil
}

// checkEnvironmentScope warns about legacy environment.* keys the frozen tree cannot carry.
func (m *migrator) checkEnvironmentScope(dir string, legacyEnv map[string]any) {
	envScope, _ := legacyEnv["environment"].(map[string]any)
	for _, key := range slices.Sorted(maps.Keys(envScope)) {
		if key != "id" && key != "applications" {
			m.warn("%s: resolved environment.%s is lost in KCL mode (the engine owns the environment scope); move it elsewhere by hand", dir, key)
		}
	}
}

// inspectDataValues resolves data values the way the legacy engine does: ytt
// --data-values-inspect over the global extra paths plus the given files.
func (m *migrator) inspectDataValues(dataFiles []string) (map[string]any, error) {
	paths := concatenate(m.g.extraYttPaths, dataFiles)
	res, err := runYttWithFilesAndStdin("migrate", paths, nil, func(name string, err error, stderr string, args []string) {
		if err != nil {
			log.Error().Str("stderr", stderr).Msg(m.g.Msg(msgRunCmd("inspect data values", name, args)))
		}
	}, "--data-values-inspect")
	if err != nil {
		return nil, err
	}
	values := map[string]any{}
	if err := yaml.Unmarshal([]byte(res.Stdout), &values); err != nil {
		return nil, fmt.Errorf("parsing ytt data-values-inspect output: %w", err)
	}
	return values, nil
}

// diffValues returns the values of want missing from or different in got, skipping the
// engine-owned environment scope. Keys present in got but absent from want cannot be
// removed by merging and are reported as warnings.
func (m *migrator) diffValues(context string, got, want map[string]any) map[string]any {
	got = maps.Clone(got)
	want = maps.Clone(want)
	delete(got, "environment")
	delete(want, "environment")

	var extra, lists []string
	patch := diffValueMaps(got, want, "", &extra, &lists)
	for _, path := range extra {
		m.warn("%s: converted value %s is absent from the legacy-resolved output and cannot be removed by merging; drop it by hand", context, path)
	}
	for _, path := range lists {
		m.warn("%s: array value %s is frozen in a patch, but ytt appends arrays over schema defaults; if the gate reports a difference here, fix it by hand", context, path)
	}
	m.patched += countLeaves(patch)
	return patch
}

func diffValueMaps(got, want map[string]any, path string, extra, lists *[]string) map[string]any {
	patch := map[string]any{}
	for key, wantValue := range want {
		keyPath := path + "." + key
		gotValue, ok := got[key]
		if !ok {
			patch[key] = wantValue
			continue
		}
		gotMap, gotIsMap := gotValue.(map[string]any)
		wantMap, wantIsMap := wantValue.(map[string]any)
		if gotIsMap && wantIsMap {
			if sub := diffValueMaps(gotMap, wantMap, keyPath, extra, lists); len(sub) > 0 {
				patch[key] = sub
			}
			continue
		}
		if !reflect.DeepEqual(gotValue, wantValue) {
			patch[key] = wantValue
			if _, isList := wantValue.([]any); isList {
				*lists = append(*lists, keyPath)
			}
		}
	}
	for key := range got {
		if _, ok := want[key]; !ok {
			*extra = append(*extra, path+"."+key)
		}
	}
	return patch
}

func countLeaves(values map[string]any) int {
	count := 0
	for _, value := range values {
		if sub, ok := value.(map[string]any); ok {
			count += countLeaves(sub)
		} else {
			count++
		}
	}
	return count
}

// mergeValues deep-merges maps left to right without mutating the inputs: maps merge
// recursively, everything else (scalars, lists, nulls) replaces — matching KCL dict-union
// semantics, which govern how the emitted tree levels combine.
func mergeValues(values ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, src := range values {
		for key, value := range src {
			if outMap, ok := out[key].(map[string]any); ok {
				if srcMap, ok := value.(map[string]any); ok {
					out[key] = mergeValues(outMap, srcMap)
					continue
				}
			}
			out[key] = value
		}
	}
	return out
}

func (m *migrator) warn(format string, args ...any) {
	m.warnings = append(m.warnings, fmt.Sprintf(format, args...))
}

func (m *migrator) printReport() {
	for _, file := range m.skipped {
		log.Warn().Msg(m.g.Msg(fmt.Sprintf(
			"Skipped %s: it contains ytt logic; its resolved values are frozen in leaf-level TODO patches", file)))
	}
	for _, warning := range m.warnings {
		log.Warn().Msg(m.g.Msg(warning))
	}
	if m.patched > 0 {
		log.Info().Msg(m.g.Msg(fmt.Sprintf(
			"Froze %d resolved value(s) in leaf-level TODO patches; turn them into KCL derivations (see docs/migration.md)", m.patched)))
	}
	log.Info().Msg(m.g.Msg(
		"Migration seed complete. Verify with the byte-identical gate (render and diff rendered/), " +
			"then hand-finish following docs/migration.md"))
}
