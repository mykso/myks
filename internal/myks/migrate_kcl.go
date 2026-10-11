package myks

import (
	"fmt"
	"maps"
	"math"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// Generated files of one environment-tree level. A KCL package is a directory, so the files
// of a level share one namespace: env.k folds the `_apps` accumulator that the per-application
// files unify into, and references the `_patch` bound by patch.k. Everything one level says
// about an application — its declaration or override plus the machine-frozen TODO values —
// lives in that application's own file, mirroring the legacy per-application data directories.
const (
	envKFileName   = "env.k"
	patchKFileName = "patch.k"
)

// appKFileName is the level file of one application. A file name is not a package path, so
// application names need no renaming; the prefix keeps them clear of env.k and patch.k.
func appKFileName(app string) string {
	return "app-" + app + ".k"
}

// appsFoldExpr folds the per-application accumulator into a plain dict. A schema instance on
// the right of `|` or `:` replaces instead of merging, which would drop everything the parent
// level said about the application.
const appsFoldExpr = "{k: v for k, v in _apps}"

// appsFoldComment explains the fold where it is written, in every level that has one.
const appsFoldComment = "\n# The comprehension folds the `_apps` schema instance into a plain dict, which merges\n" +
	"# into the inherited applications; the instance itself would replace them.\n"

// levelVarName is the level's environment data before its applications are folded in: what
// the level inherits, what it states itself, and its frozen patch. The application files of
// the level read it, so it must not depend on the `_apps` they feed.
const levelVarName = "_level"

// emit writes the seeded KCL tree: kcl.mod, main.k, and the level files of every non-empty
// environment-tree level.
func (m *migrator) emit(schemaPackage string) error {
	emitted := map[string]bool{}
	for dir, node := range m.nodes {
		emitted[dir] = m.nodeIsEmitted(node)
	}
	envKDirs := slices.DeleteFunc(slices.Sorted(maps.Keys(m.nodes)), func(dir string) bool { return !emitted[dir] })

	protos := m.declaredPrototypeSchemas()

	// The level files are rendered up front: the refusal check and the writes need the same set.
	levelFiles := map[string]string{}
	for _, dir := range envKDirs {
		node := m.nodes[dir]
		parent := node.parent
		for parent != nil && !emitted[parent.dir] {
			parent = parent.parent
		}
		files, err := m.renderNodeFiles(node, parent)
		if err != nil {
			return err
		}
		for name, content := range files {
			levelFiles[filepath.Join(m.g.RootDir, dir, name)] = content
		}
	}
	levelPaths := slices.Sorted(maps.Keys(levelFiles))

	targets := []string{
		filepath.Join(m.g.RootDir, kclModFileName),
		filepath.Join(m.g.RootDir, "main.k"),
	}
	for _, proto := range protos {
		targets = append(targets, m.protoKPath(proto))
	}
	for _, lib := range m.emittedLibs() {
		targets = append(targets, m.libKPath(m.g.RootDir, lib))
	}
	targets = append(targets, levelPaths...)
	if !m.force {
		if err := refuseExisting(targets); err != nil {
			return err
		}
	}

	if err := m.writeKclMod(schemaPackage); err != nil {
		return err
	}
	if err := m.writeMainK(); err != nil {
		return err
	}
	for _, lib := range m.emittedLibs() {
		if err := m.writeLibK(lib); err != nil {
			return fmt.Errorf("writing %s: %w", m.libKPath(m.g.RootDir, lib), err)
		}
	}
	for _, proto := range protos {
		if err := m.writeProtoK(proto); err != nil {
			return fmt.Errorf("writing %s: %w", m.protoKPath(proto), err)
		}
	}

	for _, path := range levelPaths {
		if err := writeFile(path, []byte(levelFiles[path])); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}

// renderNodeFiles renders every file of one level, keyed by file name: env.k always, patch.k
// when the level has frozen environment values, and one file per application it configures.
func (m *migrator) renderNodeFiles(node, parent *migNode) (map[string]string, error) {
	files := map[string]string{}
	render := func(name string, render func() (string, error)) error {
		content, err := render()
		if err != nil {
			return fmt.Errorf("rendering %s/%s: %w", node.dir, name, err)
		}
		files[name] = content
		return nil
	}

	if err := render(envKFileName, func() (string, error) { return m.renderEnvK(node, parent) }); err != nil {
		return nil, err
	}
	if nodeHasPatch(node) {
		if err := render(patchKFileName, func() (string, error) { return m.renderPatchK(node) }); err != nil {
			return nil, err
		}
	}
	renames := m.levelRenames(node)
	for _, name := range nodeAppNames(node) {
		if err := render(appKFileName(name), func() (string, error) { return m.renderAppK(node, name, renames[name]) }); err != nil {
			return nil, err
		}
	}
	return files, nil
}

// levelRenames renames the module-level variables an application file binds that another file
// of its level binds differently. The files of a level are one KCL package, so a name bound in
// two of them is one variable, which every reader sees with the value bound last — silently.
// env.k keeps its names; an application file's variable takes the application's name as a
// prefix (`_get_uri` becomes `_web_get_uri`).
func (m *migrator) levelRenames(node *migNode) map[string]map[string]string {
	taken := map[string][]string{}
	if node.envDerived.has() {
		taken = preludeVars(node.envDerived.prelude)
	}
	renames := map[string]map[string]string{}
	for _, name := range nodeAppNames(node) {
		decl, patch := m.appFileDerived(node, name)
		file := mergeDerivations(decl, patch)
		if !file.has() {
			continue
		}
		own := map[string]string{}
		// A variable read by a renamed one has to be compared after the rename, so the
		// renaming runs until nothing more collides.
		for changed := true; changed; {
			changed = false
			vars := preludeVars(file.renamed(own).prelude)
			for _, v := range slices.Sorted(maps.Keys(vars)) {
				if existing, ok := taken[v]; !ok || slices.Equal(existing, vars[v]) || slices.Contains(slices.Collect(maps.Values(own)), v) {
					continue
				}
				renamed := "_" + sanitizeKclIdentifier(name) + v
				for _, clash := taken[renamed]; clash || kclReservedVars[renamed]; _, clash = taken[renamed] {
					renamed += "_"
				}
				own[v] = renamed
				changed = true
				break
			}
		}
		for v, stmts := range preludeVars(file.renamed(own).prelude) {
			taken[v] = stmts
		}
		if len(own) > 0 {
			renames[name] = own
		}
	}
	return renames
}

// appFileDerived returns the derivations an application's level file states: those behind
// its declaration or override, and those of its frozen block.
func (m *migrator) appFileDerived(node *migNode, name string) (decl, patch *derivations) {
	if app, declared := node.declared[name]; declared {
		decl = m.declarationDerived(node, name, app.proto)
	} else {
		proto := protoOf(node, name)
		decl = mergeDerivations(node.protoDerived[proto], node.appDerived[name])
	}
	return decl, node.appPatchDerived[name]
}

// nodeAppNames lists, sorted, every application this level says something about.
func nodeAppNames(node *migNode) []string {
	names := map[string]bool{}
	for _, values := range []map[string]map[string]any{node.overrides, node.appPatches} {
		for name := range values {
			names[name] = true
		}
	}
	for name := range node.declared {
		names[name] = true
	}
	return slices.Sorted(maps.Keys(names))
}

func nodeHasApps(node *migNode) bool {
	return len(nodeAppNames(node)) > 0
}

func nodeHasPatch(node *migNode) bool {
	return len(node.envPatch) > 0
}

// refuseExisting keeps the migration from clobbering hand-written KCL: it is all-or-nothing,
// so a partially converted tree never gets half-overwritten by a second run.
func refuseExisting(paths []string) error {
	var existing []string
	for _, path := range paths {
		ok, err := isExist(path)
		if err != nil {
			return err
		}
		if ok {
			existing = append(existing, path)
		}
	}
	if len(existing) > 0 {
		return fmt.Errorf("refusing to overwrite existing file(s): %s; remove them or re-run the migration with --force",
			strings.Join(existing, ", "))
	}
	return nil
}

// nodeIsEmitted reports whether a level gets its own env.k. The root and the leaves always
// do; an intermediate level only when it carries content (a child package imports its
// nearest emitted ancestor, so empty levels need no file).
func (m *migrator) nodeIsEmitted(node *migNode) bool {
	return node == m.root || node.env != nil ||
		len(node.envValues)+len(node.declared)+len(node.overrides) > 0
}

// declaredPrototypeSchemas lists, sorted, the prototypes that both get a generated base
// schema and are actually declared somewhere in the tree.
func (m *migrator) declaredPrototypeSchemas() []string {
	used := map[string]bool{}
	for _, node := range m.nodes {
		for _, app := range node.declared {
			if m.protoSchemas[app.proto] != "" {
				used[app.proto] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(used))
}

// libKPath is where the KCL translation of one ytt library file lands under a package root.
func (m *migrator) libKPath(root string, lib *yttLib) string {
	return filepath.Join(root, m.g.YttLibraryDirName, lib.name+".k")
}

// writeLibK writes the KCL translation of the repo's ytt library. The whole library is one
// KCL package, so a file importing it reads every function translated out of it.
func (m *migrator) writeLibK(lib *yttLib) error {
	return writeFile(m.libKPath(m.g.RootDir, lib), []byte(strings.TrimPrefix(lib.source, "\n")))
}

func (m *migrator) protoKPath(proto string) string {
	return filepath.Join(m.g.RootDir, m.g.PrototypesDir, proto, protoKFileName)
}

// writeProtoK renders a prototype's base schema: its converted app-data values become
// attribute defaults, so applications instantiate the schema instead of repeating them, and
// a structured object value becomes a nested schema of its own (see protoSchemaPlan). Every
// attribute is optional, so a null default stays null instead of failing the required-value
// check.
func (m *migrator) writeProtoK(proto string) error {
	plan := m.protoPlans[proto]
	b := &kclWriter{derived: m.protoDerived[proto], comments: m.protoComments[proto]}
	b.WriteString("import myks\n")
	writeDerivationHeader(b, b.derived)
	plan.render(b, proto)
	b.writeTrailingComments()
	for _, failure := range plan.failed {
		m.warn("%s/%s: validation of %s is not carried into the generated KCL schema",
			m.g.PrototypesDir, proto, failure)
	}
	if b.err != nil {
		return b.err
	}
	return writeFile(m.protoKPath(proto), []byte(b.String()))
}

// migrationDocsURL is where the hand-finish steps are documented. The generated files point
// at it rather than at a repo-relative path: docs/migration.md is a file of myks, not of the
// repository being converted.
const migrationDocsURL = "https://github.com/mykso/myks/blob/main/docs/migration.md"

// pathKey addresses a value in a prototype's default tree. The separator cannot occur in a
// data key, so a key never spans two path elements.
func pathKey(path []string) string { return strings.Join(path, "\x00") }

// protoSchemaPlan is the set of KCL schemas generated for one prototype: the root schema the
// applications instantiate, plus one nested schema for every structured object value below
// it. A value the ytt schema describes with properties becomes a schema of its own, so its
// fields keep their names and types — the alternative, one `{str:any}` default per bag, hides
// every field that has no default and types none of the rest. A free-form bag (no properties
// in the ytt schema, or a key that is no KCL identifier) stays a literal default.
// An array whose ytt schema describes its element gets an element schema the same way, and
// the attribute is typed `[Element]`: KCL instantiates every element of such a list, so a
// value the element schema defaults is filled in even when the application states the array
// without it — which is what ytt did by overlaying the stated array onto the schema's.
type protoSchemaPlan struct {
	values   map[string]any
	schema   *inspectedSchema
	demanded map[string]bool   // path key -> generated without a default
	names    map[string]string // path key -> schema name; the root is the empty path
	items    map[string]string // path key of an array -> its element schema name
	taken    map[string]bool   // the schema names claimed so far
	nested   [][]string        // the generated schema paths, in generation order
	// externalSchema records that a schema document outside the prototype also governs its
	// application scope (migrator.protoSchemaOutside), so no scope can be closed.
	externalSchema bool
	// failed collects the constraints that reached no KCL expression, for the caller to report.
	failed []string
}

func newProtoSchemaPlan(root string, values map[string]any, schema *inspectedSchema) *protoSchemaPlan {
	p := &protoSchemaPlan{
		values: values,
		schema: schema,
		names:  map[string]string{"": root},
		items:  map[string]string{},
		taken:  map[string]bool{root: true},
	}
	// A data key could otherwise name a schema after a KCL literal (`Undefined`).
	for reserved := range kclReservedWords {
		p.taken[reserved] = true
	}
	if schema != nil {
		p.demanded = schema.demanded
		p.nameNested(nil, values)
	}
	return p
}

// nameNested claims a schema name for every structured object value below path, and for the
// element of every array the ytt schema describes there, depth first.
func (p *protoSchemaPlan) nameNested(path []string, values map[string]any) {
	for _, key := range p.attributes(path, values) {
		childPath := append(slices.Clone(path), key)
		if child, ok := values[key].(map[string]any); ok {
			if !p.structured(childPath, child) {
				continue
			}
			p.claimSchema(childPath)
			p.nameNested(childPath, child)
			continue
		}
		if p.structuredElement(childPath) {
			p.nameElement(childPath)
		}
	}
}

// claimSchema records a generated schema for the value at path.
func (p *protoSchemaPlan) claimSchema(path []string) string {
	name := p.claimName(path)
	p.names[pathKey(path)] = name
	p.nested = append(p.nested, path)
	return name
}

// nameElement claims the element schema of the array at path, under the array's path (which
// is how the attribute's type finds it) and under the element's own path (which is how it is
// rendered, like any other generated schema).
func (p *protoSchemaPlan) nameElement(path []string) {
	itemsPath := append(slices.Clone(path), itemsKey)
	name := p.claimSchema(itemsPath)
	p.items[pathKey(path)] = name
	p.nameBelowElement(itemsPath)
}

// nameBelowElement claims the schemas below a path the prototype's values do not reach: an
// array element has no values, only what the ytt schema describes, so the walk follows the
// schema. Whatever it describes there is typed as deeply as a value of the prototype is.
func (p *protoSchemaPlan) nameBelowElement(path []string) {
	node := p.schema.nodeAt(path)
	if node == nil {
		return
	}
	for _, key := range slices.Sorted(maps.Keys(node.Properties)) {
		childPath := append(slices.Clone(path), key)
		switch {
		case p.structuredNode(childPath):
			p.claimSchema(childPath)
			p.nameBelowElement(childPath)
		case p.structuredElement(childPath):
			p.nameElement(childPath)
		}
	}
}

// structuredElement reports whether the array at path has an element the ytt schema describes
// with fields that can all be KCL attributes.
func (p *protoSchemaPlan) structuredElement(path []string) bool {
	node := p.schema.nodeAt(path)
	if node == nil || node.Type != "array" || node.Items == nil || len(node.Items.Properties) == 0 {
		return false
	}
	for name := range node.Items.Properties {
		if !isKclIdentifier(name) {
			return false
		}
	}
	return true
}

// structuredNode reports whether the ytt schema describes the value at path as a fixed
// structure, without consulting the prototype's values: inside an array element there are
// none.
func (p *protoSchemaPlan) structuredNode(path []string) bool {
	node := p.schema.nodeAt(path)
	if node == nil || len(node.Properties) == 0 {
		return false
	}
	for name := range node.Properties {
		if !isKclIdentifier(name) {
			return false
		}
	}
	return true
}

// structured reports whether the value at path is a fixed structure the ytt schema describes,
// with fields that can all be KCL attributes.
func (p *protoSchemaPlan) structured(path []string, values map[string]any) bool {
	node := p.schema.nodeAt(path)
	if node == nil || len(node.Properties) == 0 {
		return false
	}
	names := p.attributes(path, values)
	if len(names) == 0 {
		return false
	}
	for _, name := range names {
		if !isKclIdentifier(name) {
			return false
		}
	}
	return true
}

// claimName derives a generated schema's name from the path to it: its last segment (`tls` ->
// `Tls`), lengthened with the segments above it while that name is taken
// (`ApplicationTls`), and numbered as the last resort. Every prototype is a KCL package of
// its own, so the short name is what call sites read — `webapp.Tls`.
func (p *protoSchemaPlan) claimName(path []string) string {
	var parts []string
	for _, key := range path {
		// The element of an array is named after the array, in the singular: one element of
		// `clients` is a `Client`.
		if key == itemsKey {
			if len(parts) > 0 {
				parts[len(parts)-1] = singularSchemaName(parts[len(parts)-1])
			}
			continue
		}
		parts = append(parts, schemaNamePart(key))
	}
	name := ""
	for i := len(parts) - 1; i >= 0; i-- {
		name = parts[i] + name
		if !p.taken[name] {
			p.taken[name] = true
			return name
		}
	}
	unique := name
	for i := 2; p.taken[unique]; i++ {
		unique = name + strconv.Itoa(i)
	}
	p.taken[unique] = true
	return unique
}

// schemaNamePart turns one data key into a schema name segment: `app_id` -> `AppId`. A key of
// underscores alone — ytt's convention for a private bag of helper values — is `Private`.
func schemaNamePart(key string) string {
	var name strings.Builder
	for _, word := range strings.Split(key, "_") {
		if word != "" {
			name.WriteString(strings.ToUpper(word[:1]) + word[1:])
		}
	}
	if name.Len() == 0 {
		return "Private"
	}
	return name.String()
}

// singularSchemaName names one element of an array named name. A name that does not read as
// an English plural gets an `Item` suffix instead.
func singularSchemaName(name string) string {
	for _, rule := range []struct{ plural, singular string }{
		{"ies", "y"},
		{"sses", "ss"},
		{"shes", "sh"},
		{"ches", "ch"},
		{"xes", "x"},
		{"ss", ""},
		{"us", ""},
		{"is", ""},
		{"s", ""},
	} {
		stem, ok := strings.CutSuffix(name, rule.plural)
		if !ok || stem == "" {
			continue
		}
		if rule.singular == "" && rule.plural != "s" {
			// Not a plural: `Address`, `Status`, `Analysis`.
			break
		}
		return stem + rule.singular
	}
	return name + "Item"
}

// attributes lists the KCL attributes of the schema at path: the keys the prototype's values
// carry there, plus the demanded ones pruned from those values.
func (p *protoSchemaPlan) attributes(path []string, values map[string]any) []string {
	names := slices.Collect(maps.Keys(values))
	prefix := pathKey(path)
	if prefix != "" {
		prefix += "\x00"
	}
	for key := range p.demanded {
		child, found := strings.CutPrefix(key, prefix)
		if !found || child == "" || strings.Contains(child, "\x00") {
			continue
		}
		names = append(names, child)
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// render writes the root schema and the nested ones. KCL schema definitions are
// order-independent, so the root comes first: it is what an application instantiates.
func (p *protoSchemaPlan) render(b *kclWriter, proto string) {
	p.renderSchema(b, nil, p.values, proto)
	for _, path := range p.nested {
		p.renderSchema(b, path, p.valuesAt(path), "")
	}
}

// withElementDefaults returns values with every array the generated schemas type by an element
// schema completed: KCL instantiates each element of such a list, so a field the element schema
// defaults is present even where the declaration leaves it out. The patch simulation calls this
// on the declaration's values, so it compares what KCL will produce rather than what the
// converter wrote down. A nil plan — a prototype with no generated schema, whose defaults are
// hoisted into the declaration instead — completes nothing.
//
// Nothing is filled in place: the values are shared with the prototype defaults and with the
// declaration the writer emits, neither of which may grow a value KCL supplies by itself.
func (p *protoSchemaPlan) withElementDefaults(values map[string]any, path []string) map[string]any {
	if p == nil || len(p.items) == 0 {
		return values
	}
	out := maps.Clone(values)
	for key, value := range out {
		childPath := append(slices.Clone(path), key)
		switch typed := value.(type) {
		case map[string]any:
			out[key] = p.withElementDefaults(typed, childPath)
		case []any:
			if _, ok := p.items[pathKey(childPath)]; ok {
				out[key] = p.completeElements(typed, childPath)
			}
		}
	}
	return out
}

// pruneElementDefaults removes from every frozen array the element fields that equal what its
// element schema supplies. KCL instantiates each element of a typed list, so the literal only
// has to state what differs; without this a frozen array repeats every default the prototype
// already fixed, once per element.
func (p *protoSchemaPlan) pruneElementDefaults(values map[string]any, path []string) {
	if p == nil || len(p.items) == 0 {
		return
	}
	for key, value := range values {
		childPath := append(slices.Clone(path), key)
		switch typed := value.(type) {
		case map[string]any:
			p.pruneElementDefaults(typed, childPath)
		case []any:
			if _, ok := p.items[pathKey(childPath)]; !ok {
				continue
			}
			itemsPath := append(slices.Clone(childPath), itemsKey)
			defaults := schemaDefaults(p.schema.nodeAt(itemsPath))
			for _, raw := range typed {
				element, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				for name, supplied := range defaults {
					if stated, set := element[name]; set && reflect.DeepEqual(stated, supplied) {
						delete(element, name)
					}
				}
				p.pruneElementDefaults(element, itemsPath)
			}
		}
	}
}

// completeElements copies the list at path, filling into every element the defaults its
// element schema supplies, and completing what is nested inside the element the same way: a
// schema reaches as deep as the ytt schema described the element, and so does what KCL fills
// in from it.
func (p *protoSchemaPlan) completeElements(list []any, path []string) []any {
	itemsPath := append(slices.Clone(path), itemsKey)
	defaults := schemaDefaults(p.schema.nodeAt(itemsPath))
	out := slices.Clone(list)
	for i, raw := range out {
		element, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		completed := maps.Clone(element)
		for name, value := range defaults {
			if _, set := completed[name]; !set {
				completed[name] = value
			}
		}
		out[i] = p.withElementDefaults(completed, itemsPath)
	}
	return out
}

// valuesAt returns the prototype's values at path. Below an array element there are none —
// the prototype states the array, not its elements — so the ytt schema's own defaults for the
// element stand in, which is what KCL will fill into every element written there.
func (p *protoSchemaPlan) valuesAt(path []string) map[string]any {
	if slices.Contains(path, itemsKey) {
		return schemaDefaults(p.schema.nodeAt(path))
	}
	values := p.values
	for _, key := range path {
		values, _ = values[key].(map[string]any)
	}
	return values
}

func (p *protoSchemaPlan) renderSchema(b *kclWriter, path []string, values map[string]any, proto string) {
	names := p.attributes(path, values)
	dotted := ""
	if len(path) > 0 {
		dotted = "." + strings.Join(path, ".")
	}
	b.WriteString("\n")
	if len(path) == 0 {
		b.printf("schema %s(myks.App):\n", p.names[""])
	} else {
		b.printf("schema %s:\n", p.names[pathKey(path)])
	}
	if p.openScope(path, names) {
		// KCL does not inherit an index signature into a subclass, so myks.App's has to be
		// repeated on the root: without it an application could only set the keys below.
		b.WriteString("    [...str]: any\n")
	}
	if len(path) == 0 {
		b.printf("    proto: str = %s\n", quoteKclString(proto))
	}
	for _, key := range b.sortKeys(dotted, names) {
		child := append(slices.Clone(path), key)
		childPath := "." + strings.Join(child, ".")
		b.writeComments(childPath, 4)
		if nested, ok := p.names[pathKey(child)]; ok {
			// The nested schema carries the defaults of everything below it.
			b.printf("    %s?: %s = %s {}\n", key, nested, nested)
			continue
		}
		if p.demanded[pathKey(child)] {
			// No default: the prototype validates this value without supplying a valid one.
			b.printf("    %s?: %s\n", key, p.attributeType(child, nil))
			continue
		}
		value := values[key]
		b.printf("    %s?: %s = ", key, p.attributeType(child, value))
		writeKclValue(b, value, 4, false, childPath)
		b.WriteString("\n")
	}
	if checks := p.checks(path); len(checks) > 0 {
		b.WriteString("\n    check:\n")
		for _, check := range checks {
			b.printf("        %s\n", check)
		}
	}
}

// openScope reports whether the schema at path has to take keys it does not declare. A ytt
// data-values schema is strict — it rejects a key it does not declare, whichever file states
// it — so a scope it governed needs no index signature, and the generated schema catches the
// misspelled key that ytt used to catch. The signature stays wherever that is not certain:
// no inspected schema at all, another schema document governing the same scope from the
// environment tree, a scope the schema left open, or an attribute that reached the values
// from outside the schema.
func (p *protoSchemaPlan) openScope(path, names []string) bool {
	if p.schema == nil || p.externalSchema {
		return true
	}
	node := p.schema.nodeAt(path)
	if node == nil {
		return true
	}
	if open, closed := node.AdditionalProperties.(bool); !closed || open {
		return true
	}
	for _, name := range names {
		if node.Properties[name] == nil {
			return true
		}
	}
	return false
}

// attributeType types a generated attribute from the inspected schema, which is what ytt
// enforced on the legacy data values. Where the schema says nothing (a plain data-values
// document, or a key only such a document contributed) the type is inferred from the value,
// loosely: containers keep their kind so that a `key: {...}` union merges into the default
// instead of replacing it, scalars stay `any` so an application may override with a different
// type, as plain ytt data values allowed.
func (p *protoSchemaPlan) attributeType(path []string, value any) string {
	if element, ok := p.items[pathKey(path)]; ok {
		return "[" + element + "]"
	}
	if p.schema != nil {
		if node := p.schema.nodeAt(path); node != nil {
			return kclType(node)
		}
	}
	switch value.(type) {
	case map[string]any:
		return "{str:any}"
	case []any:
		return "[any]"
	default:
		return "any"
	}
}

// checks restates as KCL check items the validations the schema at path owns: those on its own
// attributes, and those on values below it that no deeper schema covers. A value inside a
// free-form bag is reached by indexing and guarded by the keys on the way to it, so a check
// never fails on an application that replaced the bag wholesale; a value that may be null is
// guarded against its null, and one generated without a default against its absence.
//
// A KCL check runs where the schema is instantiated — where the application is declared — and
// again on every override of that instance, while ytt validated the final data values of a
// render once. That is why a value the prototype validates without supplying a satisfying
// default (`min_len=1` on an empty default, the way a prototype demands a value) is generated
// without one (pruneDemandedDefaults): unset, its check does not fire; set at any level, it is
// enforced from there on.
func (p *protoSchemaPlan) checks(path []string) []string {
	if p.schema == nil {
		return nil
	}
	var checks []string
	for _, constraint := range p.schema.constraints {
		owner, access, guards := p.locate(constraint.path, constraint.kind != constraintNotNull)
		if pathKey(owner) != pathKey(path) {
			continue
		}
		if strings.Contains(access, itemsKey) {
			// An element no generated schema describes has nowhere to carry its check.
			p.failed = append(p.failed, displayPath(constraint.path)+" (an array element with no generated schema)")
			continue
		}
		condition, requirement, err := checkCondition(access, constraint)
		if err != nil {
			p.failed = append(p.failed, fmt.Sprintf("%s (%s)", displayPath(constraint.path), err))
			continue
		}
		if len(guards) > 0 {
			condition += " if " + strings.Join(guards, " and ")
		}
		checks = append(checks, fmt.Sprintf("%s, %s", condition,
			quoteKclString(displayPath(constraint.path)+" "+requirement)))
	}
	return checks
}

// locate returns the schema that carries a constraint — the deepest generated one on the way
// to the constrained value — plus the expression reaching the value from inside it and the
// guards that expression needs.
// guardNull is false for the constraint that asserts the value is not null: guarding it
// against its own null would make the check vacuous. The nulls on the way to the value are
// still guarded either way.
func (p *protoSchemaPlan) locate(path []string, guardNull bool) (owner []string, access string, guards []string) {
	for i := 1; i < len(path); i++ {
		if _, ok := p.names[pathKey(path[:i])]; ok {
			owner = path[:i]
		}
	}
	depth := len(owner)
	access = path[depth]
	if p.demanded[pathKey(path[:depth+1])] {
		// An optional attribute left unset is Undefined, which only != compares with.
		guards = append(guards, access+" != Undefined")
	}
	for i := depth; i < len(path); i++ {
		if node := p.schema.nodeAt(path[:i+1]); node != nil && node.Nullable && (guardNull || i+1 < len(path)) {
			guards = append(guards, access+" != None")
		}
		if i+1 < len(path) {
			guards = append(guards, fmt.Sprintf("%s in %s", quoteKclString(path[i+1]), access))
			access += "[" + quoteKclString(path[i+1]) + "]"
		}
	}
	return owner, access, guards
}

// pruneDemandedDefaults removes from a prototype's default values every value its own schema
// validates but its default does not satisfy — `min_len=1` on an empty string, the way a ytt
// prototype demands a value it cannot supply. Kept, the default would fail the generated check
// at every declaration; removed, the value is absent until a level sets it, and validated from
// then on. The paths removed this way are recorded on the schema: they are still generated as
// attributes, but without a default, and their checks are guarded against the absence.
func pruneDemandedDefaults(values map[string]any, schema *inspectedSchema) {
	demanded := map[string]bool{}
	for _, constraint := range schema.constraints {
		if constraintHoldsForDefaults(values, constraint) {
			continue
		}
		scope := values
		for _, key := range constraint.path[:len(constraint.path)-1] {
			next, ok := scope[key].(map[string]any)
			if !ok {
				scope = nil
				break
			}
			scope = next
		}
		if scope == nil {
			continue
		}
		delete(scope, constraint.path[len(constraint.path)-1])
		demanded[pathKey(constraint.path)] = true
	}
	schema.demanded = demanded
}

// constraintHoldsForDefaults reports whether the prototype's own default values satisfy a
// constraint. A path that is absent counts as satisfied: the check is guarded by the same
// keys, so it does not fire either.
func constraintHoldsForDefaults(defaults map[string]any, constraint schemaConstraint) bool {
	value := any(defaults)
	for _, key := range constraint.path {
		scope, ok := value.(map[string]any)
		if !ok {
			return true
		}
		if value, ok = scope[key]; !ok {
			return true
		}
	}
	switch constraint.kind {
	case constraintNotNull:
		return value != nil
	case constraintMinLength, constraintMaxLength:
		length, ok := valueLength(value)
		if !ok {
			return false
		}
		bound, ok := constraint.value.(int)
		if !ok {
			return false
		}
		if constraint.kind == constraintMinLength {
			return length >= bound
		}
		return length <= bound
	case constraintMinimum, constraintMaximum:
		number, ok := valueNumber(value)
		bound, boundOk := valueNumber(constraint.value)
		if !ok || !boundOk {
			return false
		}
		if constraint.kind == constraintMinimum {
			return number >= bound
		}
		return number <= bound
	case constraintEnum:
		allowed, ok := constraint.value.([]any)
		if !ok {
			return false
		}
		return slices.ContainsFunc(allowed, func(candidate any) bool { return reflect.DeepEqual(candidate, value) })
	default:
		return false
	}
}

func valueLength(value any) (int, bool) {
	switch typed := value.(type) {
	case string:
		return len(typed), true
	case []any:
		return len(typed), true
	case map[string]any:
		return len(typed), true
	default:
		return 0, false
	}
}

func valueNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}

// checkCondition renders one constraint as a KCL boolean expression over the accessed value,
// with the requirement it states in words for the check message.
func checkCondition(access string, constraint schemaConstraint) (condition, requirement string, err error) {
	if constraint.kind == constraintNotNull {
		return access + " != None", "must not be null", nil
	}
	if constraint.kind == constraintEnum {
		values, ok := constraint.value.([]any)
		if !ok {
			return "", "", fmt.Errorf("enum is not a list")
		}
		rendered := make([]string, 0, len(values))
		for _, value := range values {
			scalar, err := kclScalar(value)
			if err != nil {
				return "", "", err
			}
			rendered = append(rendered, scalar)
		}
		list := strings.Join(rendered, ", ")
		return fmt.Sprintf("%s in [%s]", access, list), fmt.Sprintf("must be one of [%s]", list), nil
	}
	value := constraint.value
	// OpenAPI numbers decode as floats; an integral bound reads better as an int and compares
	// the same.
	if number, ok := value.(float64); ok && number == math.Trunc(number) {
		value = int64(number)
	}
	bound, err := kclScalar(value)
	if err != nil {
		return "", "", err
	}
	switch constraint.kind {
	case constraintMinLength:
		if bound == "1" {
			return fmt.Sprintf("len(%s) >= 1", access), "must not be empty", nil
		}
		return fmt.Sprintf("len(%s) >= %s", access, bound), "must be at least " + bound + " long", nil
	case constraintMaxLength:
		return fmt.Sprintf("len(%s) <= %s", access, bound), "must be at most " + bound + " long", nil
	case constraintMinimum:
		return fmt.Sprintf("%s >= %s", access, bound), "must be >= " + bound, nil
	case constraintMaximum:
		return fmt.Sprintf("%s <= %s", access, bound), "must be <= " + bound, nil
	default:
		return "", "", fmt.Errorf("unknown constraint kind %q", constraint.kind)
	}
}

func (m *migrator) writeKclMod(schemaPackage string) error {
	var dep string
	if strings.HasPrefix(schemaPackage, "oci://") {
		dep = fmt.Sprintf("myks = { oci = %q, tag = %q }", schemaPackage, supportedKclSchemaVersion)
	} else {
		dep = fmt.Sprintf("myks = { path = %q }", filepath.ToSlash(schemaPackage))
	}
	content := fmt.Sprintf(`[package]
name = "config"
version = "0.1.0"

[dependencies]
%s
`, dep)
	return writeFile(filepath.Join(m.g.RootDir, kclModFileName), []byte(content))
}

func (m *migrator) writeMainK() error {
	b := &kclWriter{}
	b.WriteString("# Generated by `myks migrate`: the root evaluation emitting the frozen resolved tree.\n")
	b.WriteString("# Environments are discovered from this output only — no filesystem walk.\n")
	b.printf("# The generated tree is a machine seed: see %s for the hand-finish steps.\n", migrationDocsURL)
	b.WriteString("import myks\n")

	leafDirs := slices.Sorted(maps.Keys(m.g.environments))
	refs := leafImportNames(leafDirs)
	for i, dir := range leafDirs {
		if refs[i] == filepath.Base(dir) {
			b.printf("import %s\n", packagePath(dir))
		} else {
			b.printf("import %s as %s\n", packagePath(dir), refs[i])
		}
	}

	b.WriteString("\nmyksSchemaVersion = myks.SCHEMA_VERSION\n")
	b.WriteString("environments = {\n")
	for i, dir := range leafDirs {
		rel, err := filepath.Rel(m.g.EnvironmentBaseDir, dir)
		if err != nil {
			return fmt.Errorf("resolving environment path %s: %w", dir, err)
		}
		b.printf("    %s = %s.env\n", quoteKclString(filepath.ToSlash(rel)), refs[i])
	}
	b.WriteString("}\n")

	return writeFile(filepath.Join(m.g.RootDir, "main.k"), []byte(b.String()))
}

// leafImportNames names the import of every leaf package in main.k: its directory name, which
// is what KCL binds an unaliased import to, unless another leaf, the schema package or a
// variable of main.k claims the same name; then every leaf of that name is numbered.
func leafImportNames(leafDirs []string) []string {
	count := map[string]int{"myks": 1, "environments": 1, "myksSchemaVersion": 1}
	for _, dir := range leafDirs {
		count[filepath.Base(dir)]++
	}
	names := make([]string, len(leafDirs))
	for i, dir := range leafDirs {
		names[i] = filepath.Base(dir)
		if count[names[i]] > 1 {
			names[i] = fmt.Sprintf("env_%d", i+1)
		}
	}
	return names
}

func packagePath(dir string) string {
	return strings.ReplaceAll(filepath.ToSlash(dir), "/", ".")
}

// kclWriter accumulates rendered KCL together with the first value that cannot be
// represented, so rendering fails instead of emitting a broken file.
type kclWriter struct {
	b   strings.Builder
	err error
	// derived holds the KCL translation of the ytt computation behind this file's values:
	// where it has an expression for a value's path, the file states that expression instead
	// of the value ytt resolved.
	derived *derivations
	// comments holds what the legacy files wrote above their values, keyed by dotted value
	// path, so the same note sits above the same value here.
	comments map[string][]string
}

// writeComments writes the comment block belonging to one value path, if any. An empty line of
// the block is the blank line the source separated the value with, dropped where it would
// open a block or double another blank line.
func (w *kclWriter) writeComments(path string, indent int) {
	pad := strings.Repeat(" ", indent)
	for _, line := range w.comments[path] {
		if line == "" {
			if s := w.b.String(); s != "" && !strings.HasSuffix(s, "\n\n") &&
				!strings.HasSuffix(s, "{\n") && !strings.HasSuffix(s, "[\n") && !strings.HasSuffix(s, ":\n") {
				w.WriteString("\n")
			}
			continue
		}
		w.printf("%s%s\n", pad, line)
	}
}

// writeTrailingComments ends the file with the blocks its sources ended with.
func (w *kclWriter) writeTrailingComments() {
	if lines := w.comments[trailingCommentsPath]; len(lines) > 0 {
		w.WriteString("\n")
		w.writeComments(trailingCommentsPath, 0)
	}
}

// sortKeys orders the keys of the mapping at a dotted path the way the source files stated
// them. A key no source file stated comes after those, alphabetically.
func (w *kclWriter) sortKeys(path string, keys []string) []string {
	order := w.comments[keyOrderPath(path)]
	rank := func(key string) int {
		if i := slices.Index(order, key); i >= 0 {
			return i
		}
		return len(order)
	}
	sorted := slices.Sorted(slices.Values(keys))
	slices.SortStableFunc(sorted, func(a, b string) int { return rank(a) - rank(b) })
	return sorted
}

// writeDerivationHeader writes what a file's derivations read: the imports of the translated
// ytt library, and the module-level variables of the ytt prelude behind them.
func writeDerivationHeader(b *kclWriter, d *derivations) {
	if !d.has() {
		return
	}
	for _, statement := range d.imports {
		b.printf("%s\n", statement)
	}
	if len(d.prelude) > 0 {
		b.WriteString("\n")
		for _, statement := range d.prelude {
			for _, line := range d.notes[statement] {
				b.printf("%s\n", line)
			}
			b.printf("%s\n", statement)
		}
	}
}

// WriteString drops the always-nil strings.Builder error so call sites stay readable.
func (w *kclWriter) WriteString(s string) {
	_, _ = w.b.WriteString(s)
}

func (w *kclWriter) printf(format string, args ...any) {
	w.WriteString(fmt.Sprintf(format, args...))
}

func (w *kclWriter) String() string {
	return w.b.String()
}

func (w *kclWriter) fail(err error) {
	if w.err == nil {
		w.err = err
	}
}

// renderEnvK renders one level's env.k: the level's own values plus the wiring. The root
// instantiates the base schema; every other level imports its nearest emitted ancestor and
// patches it with a dict union. The applications live in the per-application files of the
// same KCL package, folded in here from the `_apps` accumulator they unify into; the frozen
// environment values live in patch.k, referenced as `_patch`.
//
// The level's environment data is bound to `_level` — inherited values, this level's own, and
// its frozen patch — and the applications are folded in only where `env` is built from it.
// That keeps `_level` free of `_apps`, so the level's application files can read it: an
// application deriving a value from its environment is what `@myks:data.lib.yaml` did for
// the legacy ytt files, and `_level` is where that derivation reads it now. Folding the
// applications into `_level` instead would make it undefined for the very files that feed it.
func (m *migrator) renderEnvK(node, parent *migNode) (string, error) {
	b := &kclWriter{derived: node.envDerived, comments: node.envComments}

	hasApps := nodeHasApps(node)
	if node == m.root {
		b.WriteString("import myks\n")
		writeDerivationHeader(b, b.derived)
		b.WriteString("\n")
		writeAppsBase(b, node)
		rootVar := "env"
		if hasApps {
			rootVar = levelVarName
		}
		b.printf("%s = myks.Environment {\n", rootVar)
		writeKclEntries(b, node.envValues, 4, false, "")
		b.WriteString("}\n")
		if hasApps {
			b.WriteString(appsFoldComment)
			b.printf("env = %s | {applications = %s}\n", levelVarName, appsFoldExpr)
		}
		b.writeTrailingComments()
		return b.String(), b.err
	}

	if node.env != nil || hasApps {
		// The schema package is needed for finalize on a leaf and for the `_apps` accumulator.
		b.WriteString("import myks\n")
	}
	b.printf("import %s as parent\n", packagePath(parent.dir))
	writeDerivationHeader(b, b.derived)
	b.WriteString("\n")
	writeAppsBase(b, node)

	hasPatch := nodeHasPatch(node)
	levelVar := "env"
	if hasPatch || hasApps || node.env != nil {
		// A leaf wraps the level in finalize, the applications are folded onto it, and the
		// application files read it: all three need the level under a name of its own.
		levelVar = levelVarName
	}

	b.printf("%s = parent.env | {\n", levelVar)
	if node.env != nil {
		b.printf("    id = %s\n", quoteKclString(node.env.ID))
	}
	writeKclEntries(b, node.envValues, 4, true, "")
	b.WriteString("}")
	if hasPatch {
		b.WriteString(" | _patch")
	}
	b.WriteString("\n")

	expr := levelVar
	if hasApps {
		b.WriteString(appsFoldComment)
		expr = fmt.Sprintf("%s | {applications: %s}", levelVar, appsFoldExpr)
	}
	if node.env != nil {
		b.printf("env = myks.finalize(%s)\n", expr)
	} else if levelVar != "env" {
		b.printf("env = %s\n", expr)
	}
	b.writeTrailingComments()
	return b.String(), b.err
}

// writeAppsBase declares the empty accumulator the level's per-application files unify into,
// so the level keeps resolving when the last of those files is deleted by hand.
func writeAppsBase(b *kclWriter, node *migNode) {
	if nodeHasApps(node) {
		b.WriteString("_apps: myks.Apps {}\n\n")
	}
}

// renderAppK renders one level's file for one application: everything this level says about
// it — the declaration or the dict-union override of a declaration above, plus the values
// frozen from the legacy-resolved output. The blocks unify into `_apps` in file order, so the
// frozen values win over the declaration above them.
func (m *migrator) renderAppK(node *migNode, name string, renames map[string]string) (string, error) {
	b := &kclWriter{}
	app, declared := node.declared[name]
	// Inside the `_apps` block the application's name is a key, which shadows a package of
	// the same name for the rest of the block; so does a prototype package named like the
	// schema package. Only then does the schema package take an alias.
	myks := "myks"
	if name == myks || (declared && app.proto == myks) {
		myks = "m"
		b.WriteString("import myks as m\n")
	} else {
		b.WriteString("import myks\n")
	}
	if declared {
		b.comments = m.declarationComments(node, name, app.proto)
	} else {
		proto := protoOf(node, name)
		b.comments = mergeComments(node.protoComments[proto], node.appComments[name])
	}
	// The frozen block states its own derivations, which read the level variable; their
	// prelude and imports belong in the same header.
	declDerived, patchDerived := m.appFileDerived(node, name)
	b.derived = declDerived.renamed(renames)
	patchDerived = patchDerived.renamed(renames)
	// A prototype with a generated base schema is instantiated instead of myks.App: its defaults
	// come from the schema, so the declaration's values are a union on top.
	schema := ""
	if declared {
		schema = m.protoSchemas[app.proto]
		if schema != "" {
			b.printf("import %s\n", packagePath(filepath.Join(m.g.PrototypesDir, app.proto)))
		}
	}
	writeDerivationHeader(b, mergeDerivations(b.derived, patchDerived))

	b.WriteString("\n")
	blocks := 0
	separate := func() {
		if blocks > 0 {
			b.WriteString("\n")
		}
		blocks++
	}
	openBlock := func() {
		b.printf("_apps: %s.Apps {\n    %s", myks, kclKey(name))
	}

	if declared {
		constructor, declMerge := myks+".App", false
		if schema != "" {
			constructor, declMerge = app.proto+"."+schema, true
		}
		separate()
		openBlock()
		b.printf(" = %s {", constructor)
		if len(app.values) == 0 && (schema != "" || app.proto == name) {
			b.WriteString("}\n")
		} else {
			b.WriteString("\n")
			if schema == "" && app.proto != name {
				b.printf("        proto = %s\n", quoteKclString(app.proto))
			}
			writeKclEntries(b, app.values, 8, declMerge, "")
			b.WriteString("    }\n")
		}
		b.WriteString("}\n")
	}

	if override, ok := node.overrides[name]; ok {
		separate()
		openBlock()
		b.WriteString(": ")
		writeKclValue(b, override, 4, true, "")
		b.WriteString("\n}\n")
	}

	if patch, ok := node.appPatches[name]; ok {
		separate()
		if patchHasLiterals(patch, patchDerived, "") {
			writeFrozenValuesComment(b)
		}
		declDerived := b.derived
		b.derived = patchDerived
		openBlock()
		b.WriteString(": ")
		writeKclValue(b, patch, 4, true, "")
		b.WriteString("\n}\n")
		b.derived = declDerived
	}
	b.writeTrailingComments()
	return b.String(), b.err
}

// renderPatchK renders one level's patch.k: the environment values the raw conversion could
// not reproduce, frozen as literals for hand-finishing. Frozen application values live in the
// per-application files instead.
func (m *migrator) renderPatchK(node *migNode) (string, error) {
	b := &kclWriter{}
	writeFrozenValuesComment(b)
	b.WriteString("_patch = {\n")
	writeKclEntries(b, node.envPatch, 4, true, "")
	b.WriteString("}\n")
	return b.String(), b.err
}

func writeFrozenValuesComment(b *kclWriter) {
	b.WriteString("# TODO(myks migrate): the block below is frozen from the legacy-resolved output. What it\n")
	b.WriteString("# still states as a literal was computed by ytt logic the converter could not translate,\n")
	b.WriteString("# or belongs to an array ytt resolved whole. Replace it with KCL derivations\n")
	b.printf("# (see %s).\n", migrationDocsURL)
}

// writeKclEntries renders a map's entries, one per line, in source order. In merge style
// (dict-union patches) map values use `key: {...}` so nested dicts merge instead of
// replacing; everything else uses `key = value`.
func writeKclEntries(b *kclWriter, values map[string]any, indent int, merge bool, path string) {
	pad := strings.Repeat(" ", indent)
	for _, key := range b.sortKeys(path, slices.Collect(maps.Keys(values))) {
		value := values[key]
		keyPath := path + "." + key
		b.writeComments(keyPath, indent)
		if _, isMap := value.(map[string]any); isMap && merge {
			b.printf("%s%s: ", pad, kclKey(key))
		} else {
			b.printf("%s%s = ", pad, kclKey(key))
		}
		writeKclValue(b, value, indent, merge, keyPath)
		b.WriteString("\n")
	}
}

func writeKclValue(b *kclWriter, value any, indent int, merge bool, path string) {
	if expr, ok := b.derived.expr(path); ok {
		// ytt computed this value; the file states the computation instead of its result.
		b.WriteString(expr)
		return
	}
	pad := strings.Repeat(" ", indent)
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{\n")
		writeKclEntries(b, typed, indent+4, merge, path)
		b.WriteString(pad)
		b.WriteString("}")
	case []any:
		if len(typed) == 0 {
			b.WriteString("[]")
			return
		}
		if inline, ok := inlineKclList(b, typed, path); ok {
			b.WriteString(inline)
			return
		}
		b.WriteString("[\n")
		for i, element := range typed {
			if path != "" {
				b.writeComments(elementPath(path, i), indent+4)
			}
			b.WriteString(pad)
			b.WriteString("    ")
			// A list element is a fresh value, not a union; its path is its index, which is
			// how a derivation inside a frozen list is addressed.
			writeKclValue(b, element, indent+4, false, elementPath(path, i))
			b.WriteString("\n")
		}
		b.WriteString(pad)
		b.WriteString("]")
	default:
		scalar, err := kclScalar(value)
		if err != nil {
			b.fail(err)
		}
		b.WriteString(scalar)
	}
}

// kclLineWidth is the column a list is kept within when it is written on one line.
const kclLineWidth = 100

// inlineKclList renders a list of plain scalars on one line, when it fits on the current one.
// A list holding a container, a multi-line string, a derivation or a commented element keeps
// one element per line.
func inlineKclList(b *kclWriter, list []any, path string) (string, bool) {
	elements := make([]string, 0, len(list))
	for i, element := range list {
		if b.derived.hasPath(elementPath(path, i)) || (path != "" && len(b.comments[elementPath(path, i)]) > 0) {
			return "", false
		}
		switch typed := element.(type) {
		case map[string]any, []any:
			return "", false
		case string:
			if strings.Contains(typed, "\n") {
				return "", false
			}
		}
		scalar, err := kclScalar(element)
		if err != nil {
			return "", false
		}
		elements = append(elements, scalar)
	}
	inline := "[" + strings.Join(elements, ", ") + "]"
	return inline, b.column()+len(inline) <= kclLineWidth
}

// column is the length of the line written so far.
func (w *kclWriter) column() int {
	s := w.b.String()
	return len(s) - strings.LastIndex(s, "\n") - 1
}

// kclLiteral renders a value map as a KCL dict literal.
func kclLiteral(values map[string]any) (string, error) {
	b := &kclWriter{}
	writeKclValue(b, values, 0, false, "")
	return b.String(), b.err
}

// elementPath addresses one element of a list. A list rendered without a path of its own —
// a value nested inside another list element — keeps its elements unaddressed.
func elementPath(path string, i int) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s[%d]", path, i)
}

// patchHasLiterals reports whether a frozen block still states a value no derivation replaces.
func patchHasLiterals(value any, derived *derivations, path string) bool {
	if derived.hasPath(path) {
		return false
	}
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if patchHasLiterals(child, derived, path+"."+key) {
				return true
			}
		}
		return false
	case []any:
		for i, child := range typed {
			if patchHasLiterals(child, derived, elementPath(path, i)) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func kclScalar(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "None", nil
	case bool:
		if typed {
			return "True", nil
		}
		return "False", nil
	case string:
		if block, ok := kclTextBlock(typed); ok {
			return block, nil
		}
		return quoteKclString(typed), nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			// KCL has no NaN/Inf literals; YAML .nan/.inf would emit unparsable tokens.
			return "", fmt.Errorf("value %v has no KCL representation; replace it in the source data before migrating", typed)
		}
		formatted := strconv.FormatFloat(typed, 'f', -1, 64)
		if !strings.ContainsAny(formatted, ".eE") {
			formatted += ".0"
		}
		return formatted, nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", typed), nil
	default:
		// Uncommon YAML types (timestamps, binary): keep the value as a string literal.
		return quoteKclString(fmt.Sprintf("%v", typed)), nil
	}
}

// quoteKclString renders a Go string as a KCL string literal, in single quotes when that saves
// escaping its double quotes. KCL interpolates `${...}` inside string literals, so the
// sequence is escaped to keep the value literal.
func quoteKclString(s string) string {
	quoted := strconv.Quote(s)
	if strings.Contains(s, `"`) && !strings.Contains(s, "'") {
		quoted = "'" + unescapeDoubleQuotes(quoted[1:len(quoted)-1]) + "'"
	}
	return strings.ReplaceAll(quoted, "${", `\${`)
}

// kclTextBlock renders a multi-line string as a triple-quoted KCL string, its lines as they
// are: the backslash after the opening quotes continues the line, so the text starts on the
// next one. A string with a control character other than a newline or a tab stays one line of
// escapes.
func kclTextBlock(s string) (string, bool) {
	if !strings.Contains(s, "\n") {
		return "", false
	}
	for _, r := range s {
		if r != '\n' && r != '\t' && !strconv.IsPrint(r) {
			return "", false
		}
	}
	body := strings.ReplaceAll(s, `\`, `\\`)
	// A run of three quotes, or one right before the closing ones, would end the string.
	if strings.Contains(body, `"""`) || strings.HasSuffix(body, `"`) {
		body = strings.ReplaceAll(body, `"`, `\"`)
	}
	body = strings.ReplaceAll(body, "${", `\${`)
	return `"""\` + "\n" + body + `"""`, true
}

// unescapeDoubleQuotes drops the escape from every `\"` of a Go-quoted string's body, leaving
// every other escape sequence as it is.
func unescapeDoubleQuotes(body string) string {
	var out strings.Builder
	for i := 0; i < len(body); i++ {
		if body[i] == '\\' && i+1 < len(body) {
			if body[i+1] != '"' {
				out.WriteByte('\\')
			}
			i++
		}
		out.WriteByte(body[i])
	}
	return out.String()
}

func kclKey(key string) string {
	if isKclIdentifier(key) {
		return key
	}
	return quoteKclString(key)
}
