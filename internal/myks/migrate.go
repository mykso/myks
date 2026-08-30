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
// local path. force re-runs the conversion over an already converted repo: the legacy sources
// are read again even with kcl.mod present, and the generated files are overwritten.
func Migrate(g *Globe, schemaPackage string, force bool) error {
	if kclMode, err := g.isKclMode(); err != nil {
		return err
	} else if kclMode && !force {
		return fmt.Errorf("%s already exists: this repo is already in KCL mode; re-run with --force to overwrite the generated files", kclModFileName)
	}
	g.forceLegacyMode = force
	if err := g.ValidateRootDir(); err != nil {
		return err
	}
	if err := g.Init(1, nil); err != nil {
		return fmt.Errorf("initializing legacy repo: %w", err)
	}
	if len(g.environments) == 0 {
		return errors.New("no environments found, nothing to migrate")
	}

	m := &migrator{g: g, nodes: map[string]*migNode{}, protoBase: map[string]map[string]any{}, force: force}
	if err := m.buildTree(); err != nil {
		return err
	}
	if err := m.collectContributions(); err != nil {
		return err
	}
	if err := m.renamePrototypes(); err != nil {
		return err
	}
	m.planPrototypeSchemas()
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
	// protoSchemas maps a prototype to the KCL schema name generated for it in
	// prototypes/<proto>/proto.k. A prototype absent here gets no schema; its defaults are
	// hoisted into every declaration instead.
	protoSchemas map[string]string
	// skipped lists data files containing ytt logic; their values are frozen into leaf patches.
	skipped []string
	// warnings lists conditions the user must resolve by hand.
	warnings []string
	// patched counts leaf-level patched value paths.
	patched int
	// force allows overwriting the generated files of a previous run.
	force bool
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

// nonIdentifierCharRe matches every character that cannot appear in a KCL identifier.
var nonIdentifierCharRe = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// sanitizeKclIdentifier turns a directory name into the snake_case identifier myks uses for
// KCL package names: cert-manager -> cert_manager. It returns "" when no rename can help —
// a leading digit, a KCL keyword, or a name the generated level files already bind.
func sanitizeKclIdentifier(name string) string {
	sanitized := nonIdentifierCharRe.ReplaceAllString(name, "_")
	if !isKclIdentifier(sanitized) || kclGeneratedNames[sanitized] {
		return ""
	}
	return sanitized
}

// buildTree creates a migNode for every directory on the path from the environments base
// dir to each discovered environment. Directory names become KCL package path components,
// so each must be a valid KCL identifier.
func (m *migrator) buildTree() error {
	base := m.g.EnvironmentBaseDir
	// Every component of the base dir becomes a component of the generated KCL import paths.
	for component := range strings.SplitSeq(filepath.ToSlash(filepath.Clean(base)), "/") {
		if !isKclIdentifier(component) {
			return fmt.Errorf(
				"environments base dir %q contains path component %q, which is not a valid KCL identifier; rename it before migrating",
				base, component)
		}
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

// extractEnvironmentScope removes the engine-owned keys of the environment scope from a
// node's env values: the roster feeds application placement and the id comes from the
// discovered environment, both regenerated by the engine. Any other key of the scope is
// ordinary user data and stays in place.
func (m *migrator) extractEnvironmentScope(node *migNode) {
	envScope, ok := node.envValues["environment"].(map[string]any)
	if !ok {
		return
	}
	delete(node.envValues, "environment")

	extras := map[string]any{}
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
			extras[key] = value
		}
	}
	if len(extras) > 0 {
		node.envValues["environment"] = extras
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
// (`#@ load(...)`, `key: #@ expr`), a schema annotation that changes values
// (`#@schema/default`, `#@schema/nullable`), or an overlay directive that rewrites values
// instead of merging them (`#@overlay/remove`, which plain YAML parsing would keep). Plain
// document headers (`#@data/values`), pure matching hints
// (`#@overlay/match-child-defaults`) and annotations that only describe or constrain a
// value (`#@schema/validation`, `#@schema/type`, `#@schema/desc`) do not match.
var hasYttLogicRe = regexp.MustCompile(`#@[ \t]|#@schema/(default|nullable)|#@overlay/(remove|replace|append|insert)`)

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

// kclGeneratedNames are the identifiers the generated level files already bind; a prototype
// package importing under one of them would shadow it.
var kclGeneratedNames = map[string]bool{"m": true, "parent": true, "_apps": true}

// renamePrototypes gives every prototype directory a name usable as a KCL package name, so
// it can own a base schema: cert-manager becomes cert_manager. Every directory keyed by the
// prototype name moves with it — the prototype itself and the `_proto/<name>` override dirs
// of each environment level, which the render pipeline resolves by exact name.
//
// Each legacy name is kept working as a symlink to the new directory, so the legacy sources
// still resolve: `myks migrate --force` can re-read them, and the byte-identical gate can
// still render the legacy tree after the conversion.
//
// Application names are untouched: they are the keys of the generated `applications` dict,
// taken from the legacy roster, so every rendered path stays where it is. What does change
// is `myks.context.prototype`; ytt templates reading it render differently, which the gate
// reports.
func (m *migrator) renamePrototypes() error {
	for _, proto := range m.prototypeNames() {
		target := sanitizeKclIdentifier(proto)
		if target == "" || target == proto {
			// Nothing to do, or nothing a rename can fix: planPrototypeSchemas explains.
			continue
		}

		// Decide over all locations first, so a collision anywhere leaves the prototype whole.
		var toMove []string
		renamed, collision := false, false
		for _, base := range m.prototypeDirBases() {
			source, err := isRealDir(filepath.Join(base, proto))
			if err != nil {
				return err
			}
			existing, err := isRealDir(filepath.Join(base, target))
			if err != nil {
				return err
			}
			switch {
			case source && existing:
				m.warn("%s: %s is not renamed to %q for its base schema because that directory already exists; resolve the collision by hand",
					base, proto, target)
				collision = true
			case source:
				toMove = append(toMove, base)
			case existing:
				// Renamed by an earlier run; the legacy sources still name the symlink.
				renamed = true
			}
		}
		if collision || (len(toMove) == 0 && !renamed) {
			continue
		}

		for _, base := range toMove {
			if err := os.Rename(filepath.Join(base, proto), filepath.Join(base, target)); err != nil {
				return fmt.Errorf("renaming %s to %s in %s: %w", proto, target, base, err)
			}
			// A relative link stays valid wherever the repo is checked out.
			if err := os.Symlink(target, filepath.Join(base, proto)); err != nil {
				m.warn("%s: %s was renamed to %q, but the compatibility symlink could not be created (%s); the legacy sources no longer resolve, so `myks migrate --force` and legacy renders need them updated by hand",
					base, proto, target, err)
			}
		}
		if len(toMove) > 0 {
			log.Info().Strs("dirs", toMove).Msg(m.g.Msg(fmt.Sprintf(
				"Renamed prototype %s to %s so it can own a KCL base schema; myks.context.prototype changes with it", proto, target)))
		}
		m.applyPrototypeRename(proto, target)
	}
	return nil
}

// prototypeDirBases lists the directories holding one subdirectory per prototype: the
// prototypes dir itself and the `_proto/` override dir of every environment level.
func (m *migrator) prototypeDirBases() []string {
	bases := []string{filepath.Join(m.g.RootDir, m.g.PrototypesDir)}
	for _, dir := range slices.Sorted(maps.Keys(m.nodes)) {
		bases = append(bases, filepath.Join(m.g.RootDir, dir, m.g.PrototypeOverrideDir))
	}
	return bases
}

// prototypeNames lists, sorted, every prototype the conversion knows about: the directories
// under prototypes/ plus the names the legacy rosters reference (which, after an earlier
// rename, are symlinks rather than directories).
func (m *migrator) prototypeNames() []string {
	names := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(m.g.RootDir, m.g.PrototypesDir))
	if err != nil && !os.IsNotExist(err) {
		// A missing or unreadable prototypes dir is not fatal here: the rosters below still
		// name every prototype the conversion needs, and collectContributions already ran.
		log.Debug().Err(err).Msg(m.g.Msg("Unable to list the prototypes directory"))
	}
	for _, entry := range entries {
		if entry.IsDir() && !isInternalDir(entry.Name()) {
			names[entry.Name()] = true
		}
	}
	for _, env := range m.g.environments {
		for _, proto := range env.foundApplications {
			names[proto] = true
		}
	}
	return slices.Sorted(maps.Keys(names))
}

// applyPrototypeRename points every in-memory reference at the new prototype name, so the
// generated tree imports and declares it under the name it now has on disk.
func (m *migrator) applyPrototypeRename(oldName, newName string) {
	moveKey(m.protoBase, oldName, newName)
	for _, node := range m.nodes {
		moveKey(node.protoValues, oldName, newName)
		for i := range node.rawRoster {
			if node.rawRoster[i].proto == oldName {
				node.rawRoster[i].proto = newName
			}
		}
	}
	for _, env := range m.g.environments {
		for app, proto := range env.foundApplications {
			if proto == oldName {
				env.foundApplications[app] = newName
			}
		}
	}
}

func moveKey[V any](m map[string]V, oldKey, newKey string) {
	if value, ok := m[oldKey]; ok {
		m[newKey] = value
		delete(m, oldKey)
	}
}

// isRealDir reports whether path is a directory rather than a symlink to one, which is how a
// renamed prototype is told apart from the compatibility symlink left under its legacy name.
func isRealDir(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	return info.IsDir(), nil
}

// planPrototypeSchemas decides which prototypes get a generated base schema in
// prototypes/<proto>/proto.k. A prototype qualifies when its directory name is a usable KCL
// package name and its convertible app-data has top-level keys that can all become schema
// attributes. A prototype that does not qualify is not an error: its defaults keep being
// hoisted into every application declaration, as before, and the warning names the fix.
func (m *migrator) planPrototypeSchemas() {
	m.protoSchemas = map[string]string{}
	prototypesDirUsable := true
	for component := range strings.SplitSeq(filepath.ToSlash(filepath.Clean(m.g.PrototypesDir)), "/") {
		if !isKclIdentifier(component) {
			m.warn("%s: path component %q is not a valid KCL identifier, so no prototype base schemas are generated; rename it to get them",
				m.g.PrototypesDir, component)
			prototypesDirUsable = false
			break
		}
	}
	if !prototypesDirUsable {
		return
	}

	for _, proto := range slices.Sorted(maps.Keys(m.protoBase)) {
		values := m.protoBase[proto]
		if len(values) == 0 {
			continue
		}
		if !isKclIdentifier(proto) || kclGeneratedNames[proto] {
			m.warn("%s/%s: no base schema generated (the converter could not derive a KCL package name for this directory); its defaults are repeated in every application declaration instead — rename it by hand to an identifier that starts with a letter or underscore and is neither a KCL keyword nor one of %q, then migrate again",
				m.g.PrototypesDir, proto, slices.Sorted(maps.Keys(kclGeneratedNames)))
			continue
		}
		var unusable []string
		for _, key := range slices.Sorted(maps.Keys(values)) {
			// `proto` is set by the generated schema itself, so it cannot also be an attribute.
			if !isKclIdentifier(key) || key == "proto" {
				unusable = append(unusable, key)
			}
		}
		if len(unusable) > 0 {
			m.warn("%s/%s: no base schema generated (data keys unusable as KCL attributes: %s); its defaults are repeated in every application declaration instead",
				m.g.PrototypesDir, proto, strings.Join(unusable, ", "))
			continue
		}
		m.protoSchemas[proto] = kclSchemaName(proto)
	}
}

// kclSchemaName turns a prototype directory name into its schema name: web_app -> WebApp.
// The directory is a validated KCL identifier, so the result is one too.
func kclSchemaName(dir string) string {
	b := &strings.Builder{}
	for part := range strings.SplitSeq(dir, "_") {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
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
// declarationValues merges the values a declaration at node decl carries, in legacy file
// order. With a generated base schema the prototype's own defaults stay in it
// (prototypes/<proto>/proto.k) and only what the environment tree adds on top is carried;
// without one they are hoisted into the declaration.
func (m *migrator) declarationValues(decl *migNode, name, proto string) map[string]any {
	var values map[string]any
	if m.protoSchemas[proto] == "" {
		values = m.protoBase[proto]
	}
	for _, node := range decl.chain() {
		values = mergeValues(values, node.protoValues[proto])
	}
	for _, node := range decl.chain() {
		values = mergeValues(values, node.appValues[name])
	}
	return values
}

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
				decl.declared[name] = migApp{name: name, proto: proto, values: m.declarationValues(decl, name, proto)}
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
						// The generated declaration instantiates the prototype's schema, so the
						// simulated values start from that schema's defaults.
						treeApp = mergeValues(m.protoBase[decl.proto], decl.values)
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
	got = withoutEngineEnvKeys(got)
	want = withoutEngineEnvKeys(want)

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

// withoutEngineEnvKeys drops the engine-owned keys of the environment scope: the id and the
// application roster are regenerated by the engine and must never end up in a patch. The rest
// of the scope is ordinary user data and is diffed like any other value.
func withoutEngineEnvKeys(values map[string]any) map[string]any {
	values = maps.Clone(values)
	scope, _ := values["environment"].(map[string]any)
	scope = maps.Clone(scope)
	delete(scope, "id")
	delete(scope, "applications")
	if len(scope) == 0 {
		delete(values, "environment")
	} else {
		values["environment"] = scope
	}
	return values
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
