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
	for _, name := range nodeAppNames(node) {
		if err := render(appKFileName(name), func() (string, error) { return m.renderAppK(node, name) }); err != nil {
			return nil, err
		}
	}
	return files, nil
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

func (m *migrator) protoKPath(proto string) string {
	return filepath.Join(m.g.RootDir, m.g.PrototypesDir, proto, protoKFileName)
}

// writeProtoK renders a prototype's base schema: its converted app-data values become
// attribute defaults, so applications instantiate the schema instead of repeating them.
// Every attribute is optional, so a null default stays null instead of failing the
// required-value check.
func (m *migrator) writeProtoK(proto string) error {
	values := m.protoBase[proto]
	b := &kclWriter{}
	b.WriteString("# Generated by `myks migrate` from the prototype's legacy app-data files.\n")
	b.printf("# Default configuration of the %s prototype; applications instantiate this schema\n", proto)
	b.WriteString("# and override only what they change. See docs/migration.md.\n")
	b.WriteString("import myks as m\n\n")
	b.printf("schema %s(m.App):\n", m.protoSchemas[proto])
	// KCL does not inherit an index signature into a subclass, so m.App's has to be repeated:
	// without it an application could only set keys the prototype's app-data already had.
	b.WriteString("    [...str]: any\n")
	b.printf("    proto: str = %s\n", quoteKclString(proto))
	schema := m.protoInspected[proto]
	for _, key := range slices.Sorted(maps.Keys(values)) {
		value := values[key]
		b.printf("    %s?: %s = ", key, attributeType(schema, key, value))
		writeKclValue(b, value, 4, false)
		b.WriteString("\n")
	}
	if checks := m.kclChecks(proto, schema); len(checks) > 0 {
		b.WriteString("\n    check:\n")
		for _, check := range checks {
			b.printf("        %s\n", check)
		}
	}
	if b.err != nil {
		return b.err
	}
	return writeFile(m.protoKPath(proto), []byte(b.String()))
}

// attributeType types a generated attribute from the inspected schema, which is what ytt
// enforced on the legacy data values. Without an inspected schema (a plain data-values
// document) the type is inferred from the value, loosely: containers keep their kind so that
// a `key: {...}` union merges into the default instead of replacing it, scalars stay `any`
// so an application may override with a different type, as plain ytt data values allowed.
func attributeType(schema *inspectedSchema, key string, value any) string {
	if schema != nil {
		if kclType, ok := schema.types[key]; ok {
			return kclType
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

// kclChecks restates the schema's validations as KCL check items. A nested value is reached
// by indexing and guarded by the keys on the way to it, so a check never fails on an
// application that replaced the scope wholesale; a nullable attribute is guarded against
// its own null default.
//
// A KCL check runs where the schema is instantiated — where the application is declared —
// while ytt validated the final data values of a render. A validation the prototype's own
// defaults do not satisfy (`min_len=1` on an empty default, the way a prototype demands a
// value) would therefore fail on every declaration that fills it in later, so it is reported
// instead of generated.
func (m *migrator) kclChecks(proto string, schema *inspectedSchema) []string {
	if schema == nil {
		return nil
	}
	var checks []string
	for _, constraint := range schema.constraints {
		if !constraintHoldsForDefaults(m.protoBase[proto], constraint) {
			m.warn("%s/%s: validation %s of %s is not carried into the generated KCL schema: the prototype's own default does not satisfy it, so the generated check would fail wherever an application is declared; restate it where the value is set",
				m.g.PrototypesDir, proto, constraint.kind, strings.Join(constraint.path, "."))
			continue
		}
		access := constraint.path[0]
		var guards []string
		for _, key := range constraint.path[1:] {
			guards = append(guards, fmt.Sprintf("%s in %s", quoteKclString(key), access))
			access += "[" + quoteKclString(key) + "]"
		}
		if schema.nullable[constraint.path[0]] {
			guards = append(guards, access+" != None")
		}
		condition, requirement, err := checkCondition(access, constraint)
		if err != nil {
			m.warn("%s/%s: validation %s of %s is not carried into the generated KCL schema (%s)",
				m.g.PrototypesDir, proto, constraint.kind, strings.Join(constraint.path, "."), err)
			continue
		}
		if len(guards) > 0 {
			condition += " if " + strings.Join(guards, " and ")
		}
		checks = append(checks, fmt.Sprintf("%s, %s", condition,
			quoteKclString(strings.Join(constraint.path, ".")+" "+requirement)))
	}
	return checks
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
	b.WriteString("import myks\n")

	leafDirs := slices.Sorted(maps.Keys(m.g.environments))
	for i, dir := range leafDirs {
		b.printf("import %s as env_%d\n", packagePath(dir), i+1)
	}

	b.WriteString("\nmyksSchemaVersion = myks.SCHEMA_VERSION\n")
	b.WriteString("environments = {\n")
	for i, dir := range leafDirs {
		rel, err := filepath.Rel(m.g.EnvironmentBaseDir, dir)
		if err != nil {
			return fmt.Errorf("resolving environment path %s: %w", dir, err)
		}
		b.printf("    %s = env_%d.env\n", quoteKclString(filepath.ToSlash(rel)), i+1)
	}
	b.WriteString("}\n")

	return writeFile(filepath.Join(m.g.RootDir, "main.k"), []byte(b.String()))
}

func packagePath(dir string) string {
	return strings.ReplaceAll(filepath.ToSlash(dir), "/", ".")
}

// kclWriter accumulates rendered KCL together with the first value that cannot be
// represented, so rendering fails instead of emitting a broken file.
type kclWriter struct {
	b   strings.Builder
	err error
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

// writeGeneratedHeader writes the header every generated level file carries.
func writeGeneratedHeader(b *kclWriter) {
	b.WriteString("# Generated by `myks migrate` from the legacy ytt data-values files.\n")
	b.WriteString("# This is a machine seed: see docs/migration.md for the hand-finish steps.\n")
}

// renderEnvK renders one level's env.k: the level's own values plus the wiring. The root
// instantiates the base schema; every other level imports its nearest emitted ancestor and
// patches it with a dict union. The applications live in the per-application files of the
// same KCL package, folded in here from the `_apps` accumulator they unify into; the frozen
// environment values live in patch.k, referenced as `_patch`.
func (m *migrator) renderEnvK(node, parent *migNode) (string, error) {
	b := &kclWriter{}
	writeGeneratedHeader(b)

	if node == m.root {
		b.WriteString("import myks as m\n\n")
		writeAppsBase(b, node)
		b.WriteString("env = m.Environment {\n")
		writeKclEntries(b, node.envValues, 4, false)
		if nodeHasApps(node) {
			b.printf("    applications = %s\n", appsFoldExpr)
		}
		b.WriteString("}\n")
		return b.String(), b.err
	}

	if node.env != nil || nodeHasApps(node) {
		// The schema package is needed for finalize on a leaf and for the `_apps` accumulator.
		b.WriteString("import myks as m\n")
	}
	b.printf("import %s as parent\n\n", packagePath(parent.dir))
	writeAppsBase(b, node)

	hasPatch := nodeHasPatch(node)
	levelVar := "env"
	if hasPatch || node.env != nil {
		// A leaf wraps the level in finalize, so the union needs its own name.
		levelVar = "_lvl"
	}

	b.printf("%s = parent.env | {\n", levelVar)
	if node.env != nil {
		b.printf("    id = %s\n", quoteKclString(node.env.ID))
	}
	writeKclEntries(b, node.envValues, 4, true)
	if nodeHasApps(node) {
		b.printf("    applications: %s\n", appsFoldExpr)
	}
	b.WriteString("}\n")

	expr := levelVar
	if hasPatch {
		expr = levelVar + " | _patch"
	}
	if node.env != nil {
		b.printf("env = m.finalize(%s)\n", expr)
	} else if hasPatch {
		b.printf("env = %s\n", expr)
	}
	return b.String(), b.err
}

// writeAppsBase declares the empty accumulator the level's per-application files unify into,
// so the level keeps resolving when the last of those files is deleted by hand.
func writeAppsBase(b *kclWriter, node *migNode) {
	if nodeHasApps(node) {
		b.WriteString("_apps: m.Apps {}\n\n")
	}
}

// renderAppK renders one level's file for one application: everything this level says about
// it — the declaration or the dict-union override of a declaration above, plus the values
// frozen from the legacy-resolved output. The blocks unify into `_apps` in file order, so the
// frozen values win over the declaration above them.
func (m *migrator) renderAppK(node *migNode, name string) (string, error) {
	b := &kclWriter{}
	writeGeneratedHeader(b)
	b.WriteString("import myks as m\n")

	app, declared := node.declared[name]
	// A prototype with a generated base schema is instantiated instead of m.App: its defaults
	// come from the schema, so the declaration's values are a union on top.
	schema := ""
	if declared {
		schema = m.protoSchemas[app.proto]
		if schema != "" {
			b.printf("import %s\n", packagePath(filepath.Join(m.g.PrototypesDir, app.proto)))
		}
	}

	b.WriteString("\n")
	blocks := 0
	separate := func() {
		if blocks > 0 {
			b.WriteString("\n")
		}
		blocks++
	}
	openBlock := func() {
		b.printf("_apps: m.Apps {\n    %s", kclKey(name))
	}

	if declared {
		constructor, declMerge := "m.App", false
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
			writeKclEntries(b, app.values, 8, declMerge)
			b.WriteString("    }\n")
		}
		b.WriteString("}\n")
	}

	if override, ok := node.overrides[name]; ok {
		separate()
		openBlock()
		b.WriteString(": ")
		writeKclValue(b, override, 4, true)
		b.WriteString("\n}\n")
	}

	if patch, ok := node.appPatches[name]; ok {
		separate()
		writeFrozenValuesComment(b)
		openBlock()
		b.WriteString(": ")
		writeKclValue(b, patch, 4, true)
		b.WriteString("\n}\n")
	}
	return b.String(), b.err
}

// renderPatchK renders one level's patch.k: the environment values the raw conversion could
// not reproduce, frozen as literals for hand-finishing. Frozen application values live in the
// per-application files instead.
func (m *migrator) renderPatchK(node *migNode) (string, error) {
	b := &kclWriter{}
	writeGeneratedHeader(b)
	b.WriteString("#\n")
	writeFrozenValuesComment(b)
	b.WriteString("_patch = {\n")
	writeKclEntries(b, node.envPatch, 4, true)
	b.WriteString("}\n")
	return b.String(), b.err
}

func writeFrozenValuesComment(b *kclWriter) {
	b.WriteString("# TODO(myks migrate): the values below were computed by ytt logic that the converter\n")
	b.WriteString("# cannot translate to KCL; they are frozen here as literals from the legacy-resolved\n")
	b.WriteString("# output. Replace them with KCL derivations (see docs/migration.md).\n")
}

// writeKclEntries renders a map's entries, one per line, keys sorted. In merge style
// (dict-union patches) map values use `key: {...}` so nested dicts merge instead of
// replacing; everything else uses `key = value`.
func writeKclEntries(b *kclWriter, values map[string]any, indent int, merge bool) {
	pad := strings.Repeat(" ", indent)
	for _, key := range slices.Sorted(maps.Keys(values)) {
		value := values[key]
		if _, isMap := value.(map[string]any); isMap && merge {
			b.printf("%s%s: ", pad, kclKey(key))
		} else {
			b.printf("%s%s = ", pad, kclKey(key))
		}
		writeKclValue(b, value, indent, merge)
		b.WriteString("\n")
	}
}

func writeKclValue(b *kclWriter, value any, indent int, merge bool) {
	pad := strings.Repeat(" ", indent)
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{\n")
		writeKclEntries(b, typed, indent+4, merge)
		b.WriteString(pad)
		b.WriteString("}")
	case []any:
		if len(typed) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		for _, element := range typed {
			b.WriteString(pad)
			b.WriteString("    ")
			// List elements are fresh values, not unions: always assign style.
			writeKclValue(b, element, indent+4, false)
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

// quoteKclString renders a Go string as a KCL string literal. KCL interpolates `${...}`
// inside string literals, so the sequence is escaped to keep the value literal.
func quoteKclString(s string) string {
	return strings.ReplaceAll(strconv.Quote(s), "${", `\${`)
}

func kclKey(key string) string {
	if isKclIdentifier(key) {
		return key
	}
	return quoteKclString(key)
}
