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
	m.translateYttLibrary()
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
	// protoDerived mirrors protoBase with the KCL translation of what ytt computed in it.
	protoDerived map[string]*derivations
	// protoComments holds what the prototype's app-data files wrote above their values.
	protoComments map[string]map[string][]string
	// protoInspected holds, per prototype, what its app-data schema document declares:
	// attribute types and validations, which the generated base schema restates.
	protoInspected map[string]*inspectedSchema
	// protoSchemaOutside names the prototypes whose application scope a schema document
	// outside the prototype directory also governs — one in a `_proto/<proto>/` or
	// `_apps/<app>/` directory of the environment tree. What such a document declares is not
	// in protoInspected, so the prototype's schemas cannot be closed against it.
	protoSchemaOutside map[string]bool
	// protoSchemas maps a prototype to the KCL schema name generated for it in
	// prototypes/<proto>/proto.k. A prototype absent here gets no schema; its defaults are
	// hoisted into every declaration instead.
	protoSchemas map[string]string
	// protoPlans holds the schemas planned for each prototype in protoSchemas. The plan is what
	// proto.k is rendered from, and what the patch simulation consults for the defaults KCL
	// supplies on its own.
	protoPlans map[string]*protoSchemaPlan
	// skipped lists the data files the conversion could not carry over in full; their values
	// are frozen into leaf patches.
	skipped []skippedFile
	// resolved lists the data files ytt resolved standalone, with the values its Starlark
	// computed that no KCL derivation was found for: they are converted as the literals ytt
	// produced, which is the whole of their meaning — such a file reads nothing outside
	// itself, so a literal cannot go stale.
	resolved []skippedFile
	// libs holds the KCL translation of the repo's ytt library, keyed by file stem, and
	// libPackage is the KCL package path it is imported under. libImported records whether
	// any derivation calls into it, which is what decides that the translation is written out.
	libs        map[string]*yttLib
	libPackage  string
	libImported bool
	// warnings lists conditions the user must resolve by hand.
	warnings []string
	// patched counts leaf-level patched value paths.
	patched int
	// derivedCount counts the ytt-computed values carried over as KCL derivations.
	derivedCount int
	// frozenLists collects the array values a patch freezes, reported once it is known which
	// of them a derivation replaced.
	frozenLists []frozenList
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
	// envDerived, protoDerived and appDerived mirror the value maps above with the KCL
	// translation of the ytt computation behind them.
	envDerived   *derivations
	protoDerived map[string]*derivations
	appDerived   map[string]*derivations
	// envComments, protoComments and appComments mirror them with what those files wrote
	// above their values.
	envComments   map[string][]string
	protoComments map[string]map[string][]string
	appComments   map[string]map[string][]string
	// protoContextual and appContextual hold the candidate translations of what those files
	// compute from the environment's data values; appPatchDerived holds, per application, the
	// ones proven at this leaf, which the frozen block states instead of the literal.
	protoContextual map[string]*derivations
	appContextual   map[string]*derivations
	appPatchDerived map[string]*derivations
}

type migApp struct {
	name, proto string
	values      map[string]any
}

// frozenList is one array value a leaf patch froze, with the context to report it under.
type frozenList struct{ context, path string }

// skippedFile is one data file the conversion could not carry over in full: deferred lists
// the value paths left to the leaf patches, or is empty when the whole file was.
type skippedFile struct {
	file     string
	deferred []string
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

		protoDerived:    map[string]*derivations{},
		appDerived:      map[string]*derivations{},
		protoContextual: map[string]*derivations{},
		appContextual:   map[string]*derivations{},
		appPatchDerived: map[string]*derivations{},
	}
	m.nodes[dir] = node
	return node
}

// collectContributions raw-converts the data-values files of every node (and, at the root,
// of prototypes/). Files containing ytt logic are recorded and skipped: their effect is
// frozen into leaf patches later.
func (m *migrator) collectContributions() error {
	cfg := &m.g.Config
	m.protoSchemaOutside = map[string]bool{}
	appSchemas := map[string]bool{}
	for _, dir := range slices.Sorted(maps.Keys(m.nodes)) {
		node := m.nodes[dir]
		envData, err := m.convertFileGlob(filepath.Join(node.dir, cfg.EnvironmentDataFileName))
		if err != nil {
			return err
		}
		node.envValues = envData.values
		node.envDerived = envData.derived
		node.envComments = envData.comments
		m.extractEnvironmentScope(node)

		protoOverrides, err := m.convertPerDirGlobs(filepath.Join(node.dir, cfg.PrototypeOverrideDir), cfg.ApplicationDataFileName)
		if err != nil {
			return err
		}
		for proto, converted := range protoOverrides {
			if converted.schema != nil {
				m.protoSchemaOutside[proto] = true
			}
		}
		node.protoValues = valuesOf(protoOverrides)
		node.protoDerived = derivedOf(protoOverrides)
		node.protoComments = commentsOf(protoOverrides)
		node.protoContextual = contextualOf(protoOverrides)
		apps, err := m.convertPerDirGlobs(filepath.Join(node.dir, cfg.AppsDir), cfg.ApplicationDataFileName)
		if err != nil {
			return err
		}
		for app, converted := range apps {
			if converted.schema != nil {
				appSchemas[app] = true
			}
		}
		node.appValues = valuesOf(apps)
		node.appDerived = derivedOf(apps)
		node.appComments = commentsOf(apps)
		node.appContextual = contextualOf(apps)
	}

	// An `_apps/<app>/` schema document governs the scope of whatever prototype that
	// application uses, so the rosters are what resolves it to a prototype.
	for _, node := range m.nodes {
		for _, entry := range node.rawRoster {
			if appSchemas[entry.name] {
				m.protoSchemaOutside[entry.proto] = true
			}
		}
	}

	prototypes, err := m.convertPerDirGlobs(filepath.Join(m.g.RootDir, cfg.PrototypesDir), cfg.ApplicationDataFileName)
	if err != nil {
		return err
	}
	m.protoBase = valuesOf(prototypes)
	m.protoDerived = derivedOf(prototypes)
	m.protoComments = commentsOf(prototypes)
	m.protoInspected = map[string]*inspectedSchema{}
	for proto, converted := range prototypes {
		if converted.schema != nil {
			m.protoInspected[proto] = converted.schema
			pruneDemandedDefaults(m.protoBase[proto], converted.schema)
		}
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
		case envApplicationsKey:
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

// convertedFile is the conversion of one or more data-values files: the values, plus what
// the schema documents among them declare (nil when none did).
type convertedFile struct {
	values map[string]any
	schema *inspectedSchema
	// derived holds the KCL translation of the ytt computation of the file, so the generated
	// file states the derivation instead of the value it produced.
	derived *derivations
	// contextual holds the translation of what the file computes from the data values of its
	// environment. Those values are frozen per leaf, so the translation is a candidate only:
	// it is kept at a leaf where KCL evaluates it to exactly the value frozen there
	// (proveContextual), and dropped at every other.
	contextual *derivations
	// comments holds what the file wrote above its values, keyed by dotted value path, so the
	// generated file can write it above the same value.
	comments map[string][]string
}

// mergeComments folds comment blocks together, the later file winning a path both state.
func mergeComments(parts ...map[string][]string) map[string][]string {
	merged := map[string][]string{}
	for _, part := range parts {
		maps.Copy(merged, part)
	}
	return merged
}

// convertFileGlob converts all files matching the glob into one merged result. A schema
// document's values are its defaults, which a plain data-values document of the same directory
// overrides whichever way the two sort — so the schemas merge first, as ytt resolves them.
func (m *migrator) convertFileGlob(pattern string) (*convertedFile, error) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing %s: %w", pattern, err)
	}
	converted := make([]*convertedFile, 0, len(files))
	for _, file := range files {
		file, err := m.convertDataFile(file)
		if err != nil {
			return nil, err
		}
		if file != nil {
			converted = append(converted, file)
		}
	}
	merged := &convertedFile{values: map[string]any{}}
	parts := make([]*derivations, 0, len(converted))
	contextual := make([]*derivations, 0, len(converted))
	for _, schemaFirst := range []bool{true, false} {
		for _, file := range converted {
			if (file.schema != nil) != schemaFirst {
				continue
			}
			merged.values = mergeValues(merged.values, file.values)
			merged.comments = mergeComments(merged.comments, file.comments)
			merged.schema = mergeInspectedSchemas(merged.schema, file.schema)
			parts = append(parts, file.derived)
			contextual = append(contextual, file.contextual)
		}
	}
	merged.derived = mergeDerivations(parts...)
	merged.contextual = mergeDerivations(contextual...)
	return merged, nil
}

// convertPerDirGlobs converts <base>/<name>/<filePattern> for every subdirectory of base,
// returning a map keyed by subdirectory name.
func (m *migrator) convertPerDirGlobs(base, filePattern string) (map[string]*convertedFile, error) {
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*convertedFile{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", base, err)
	}
	result := map[string]*convertedFile{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		converted, err := m.convertFileGlob(filepath.Join(base, entry.Name(), filePattern))
		if err != nil {
			return nil, err
		}
		if len(converted.values) > 0 {
			result[entry.Name()] = converted
		}
	}
	return result, nil
}

// contextualOf keeps the candidate translations of a per-directory conversion.
func contextualOf(converted map[string]*convertedFile) map[string]*derivations {
	contextual := make(map[string]*derivations, len(converted))
	for name, file := range converted {
		if file.contextual.has() {
			contextual[name] = file.contextual
		}
	}
	return contextual
}

// derivedOf keeps the KCL translations of a per-directory conversion.
func derivedOf(converted map[string]*convertedFile) map[string]*derivations {
	derived := make(map[string]*derivations, len(converted))
	for name, file := range converted {
		if file.derived.has() {
			derived[name] = file.derived
		}
	}
	return derived
}

// commentsOf keeps the comments of a per-directory conversion.
func commentsOf(converted map[string]*convertedFile) map[string]map[string][]string {
	comments := make(map[string]map[string][]string, len(converted))
	for name, file := range converted {
		if len(file.comments) > 0 {
			comments[name] = file.comments
		}
	}
	return comments
}

// valuesOf drops the schema half of a per-directory conversion.
func valuesOf(converted map[string]*convertedFile) map[string]map[string]any {
	values := make(map[string]map[string]any, len(converted))
	for name, file := range converted {
		values[name] = file.values
	}
	return values
}

var (
	// renderContextRe detects a data file whose computation reads the render context — the
	// merged data values, or the environment library myks generates per application. Such a
	// file cannot be resolved on its own: what it would answer standalone is not what it
	// answers at render time.
	renderContextRe = regexp.MustCompile(`@ytt:data|@myks:`)
	// schemaDocRe detects a data-values schema document. ytt forbids mixing schema and plain
	// data-values documents in one file, so one match settles how the whole file is read.
	schemaDocRe = regexp.MustCompile(`(?m)^#@data/values-schema\b`)
)

// mappedValidationKwargs are the `#@schema/validation` keyword arguments the generated KCL
// schema restates: those ytt reports in its OpenAPI output, plus `not_null`, which the
// converter reads from the annotation itself (yttValidations).
var mappedValidationKwargs = map[string]bool{
	"min_len": true, "max_len": true, "min": true, "max": true, "one_of": true, "not_null": true,
}

// carryValidations completes the inspected schema with the validations ytt's OpenAPI output
// drops, and reports the ones that reach no KCL expression.
//
// `not_null` becomes a constraint like any other; a custom rule (a lambda or a named function)
// and a keyword argument with no counterpart cannot be translated, so the warning names the
// path of each, which is where the check has to be written by hand.
func (m *migrator) carryValidations(file string, content []byte, schema *inspectedSchema) {
	validations, err := yttValidations(content)
	if err != nil {
		log.Debug().Err(err).Msg(m.g.Msg("Reading the schema validations of " + file))
		return
	}
	var lost []string
	for _, validation := range validations {
		for _, kwarg := range validation.kwargs {
			switch {
			case kwarg == "not_null":
				schema.constraints = append(schema.constraints,
					schemaConstraint{path: validation.path, kind: constraintNotNull})
			case mappedValidationKwargs[kwarg]:
			case kwarg == "":
				lost = append(lost, strings.Join(validation.path, ".")+" (custom rule)")
			default:
				lost = append(lost, strings.Join(validation.path, ".")+" ("+kwarg+")")
			}
		}
	}
	sortConstraints(schema.constraints)
	if len(lost) > 0 {
		m.warn("%s: schema validations not carried into the generated KCL schema: %s; restate them in its check block by hand",
			file, strings.Join(unique(lost), ", "))
	}
}

// mergeInspectedSchemas folds the schema documents of one directory together, later files
// winning, the way their values are merged.
func mergeInspectedSchemas(base, next *inspectedSchema) *inspectedSchema {
	if base == nil {
		return next
	}
	if next == nil {
		return base
	}
	base.root = mergeOpenapiNodes(base.root, next.root)
	base.constraints = append(base.constraints, next.constraints...)
	return base
}

// convertDataFile converts one data-values file.
//
// A file with no ytt computation is read for what it says (convertPlainFile). A file that
// computes values is asked of ytt when it resolves on its own, and split otherwise, so the
// values it does state plainly are still converted (convertComputedFile).
func (m *migrator) convertDataFile(file string) (*convertedFile, error) {
	content, err := os.ReadFile(file) // #nosec G304 -- paths come from globbing the repo being migrated
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", file, err)
	}
	isSchema := schemaDocRe.Match(content)
	var converted *convertedFile
	if fileComputes(content) {
		converted, err = m.convertComputedFile(file, content, isSchema)
	} else {
		converted, err = m.convertPlainFile(file, content, isSchema)
	}
	if err != nil || converted == nil {
		return converted, err
	}
	converted.comments = m.readComments(file, content)
	return converted, nil
}

// readComments reads what a data-values file wrote above its values. A file the parser cannot
// read yields none: the conversion of its values has its own error path, and a missing comment
// is not worth failing one over.
func (m *migrator) readComments(file string, content []byte) map[string][]string {
	comments, err := yttComments(content)
	if err != nil {
		log.Debug().Err(err).Msg(m.g.Msg("Reading the comments of " + file))
		return nil
	}
	out := make(map[string][]string, len(comments))
	for _, comment := range comments {
		if len(comment.path) == 0 {
			m.warn("%s: the comment block the file ends with sits above no value, so it is not carried into the generated KCL; move it by hand: %s",
				file, strings.Join(comment.lines, " "))
			continue
		}
		out["."+strings.Join(comment.path, ".")] = comment.lines
	}
	return out
}

// convertPlainFile converts content that ytt computes nothing in: a schema document through
// ytt, so its defaults carry the schema semantics plain YAML parsing cannot see (an array
// defaults to empty unless annotated, `#@schema/default` wins over the written value, a
// nullable key defaults to null); a data-values document as the YAML it is.
//
// A schema document ytt cannot inspect standalone is skipped (nil result) and recorded: only
// the legacy-resolved output can settle it.
func (m *migrator) convertPlainFile(file string, content []byte, isSchema bool) (*convertedFile, error) {
	if isSchema {
		schema, err := m.inspectSchema(file, content)
		if err != nil {
			log.Debug().Err(err).Msg(m.g.Msg("Falling back to freezing " + file))
			m.skip(file, nil)
			return nil, nil
		}
		m.carryValidations(file, content, schema)
		return &convertedFile{values: schema.defaults, schema: schema}, nil
	}

	values := map[string]any{}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	for {
		var doc map[string]any
		if err := decoder.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parsing %s: %w", file, err)
		}
		values = mergeValues(values, doc)
	}
	return &convertedFile{values: values}, nil
}

// convertComputedFile converts a file whose values ytt computes.
//
// When the file resolves on its own — its Starlark reads nothing but itself and the repo's
// ytt library — ytt is asked for the answer, and the computed values are converted as
// literals where the file sits instead of being frozen, per leaf, at the far end of the tree.
//
// Otherwise the file is split (splitYttFile): what it states plainly is converted like any
// other file, and only the computed values are left to the leaf patches. A file with nothing
// left after the split is skipped (nil result) and recorded.
func (m *migrator) convertComputedFile(file string, content []byte, isSchema bool) (*convertedFile, error) {
	// A file whose computation needs the render context (the merged data values, the
	// environment library myks generates per application) answers standalone something other
	// than what it answers at render time, so ytt is not asked.
	if !renderContextRe.Match(content) {
		converted, err := m.resolveStandalone(file, content, isSchema)
		if err == nil {
			m.deriveStandalone(file, content, converted)
			return converted, nil
		}
		log.Debug().Err(err).Msg(m.g.Msg("Resolving " + file + " standalone failed; splitting it"))
	}

	split, err := splitYttFile(content)
	if err != nil || split.kept == 0 {
		if err != nil {
			log.Debug().Err(err).Msg(m.g.Msg("Falling back to freezing " + file))
		}
		m.skip(file, nil)
		return nil, nil
	}
	converted, err := m.convertPlainFile(file, split.sanitized, isSchema)
	if err != nil || converted == nil {
		// convertPlainFile has recorded the whole-file skip already.
		return converted, err
	}
	if len(split.deferred) > 0 {
		m.skip(file, split.deferred)
		// The values left out are computed from the environment's data values, which only a
		// leaf has. The translation is proven leaf by leaf, against what was frozen there.
		converted.contextual = yttDerivations(file, content, m.libs, m.libPackage, levelVarName)
	}
	return converted, nil
}

// resolveStandalone asks ytt for the values of a file that resolves on its own.
func (m *migrator) resolveStandalone(file string, content []byte, isSchema bool) (*convertedFile, error) {
	if isSchema {
		schema, err := m.inspectSchema(file, content)
		if err != nil {
			return nil, err
		}
		m.carryValidations(file, content, schema)
		return &convertedFile{values: schema.defaults, schema: schema}, nil
	}
	values, err := m.resolveDataValues(file, content)
	if err != nil {
		return nil, err
	}
	return &convertedFile{values: values}, nil
}

// skip records a file the conversion could not carry over in full: with no paths, the whole
// file; with paths, the values inside it that only ytt can produce.
func (m *migrator) skip(file string, deferred []string) {
	m.skipped = append(m.skipped, skippedFile{file: file, deferred: deferred})
}

// translateYttLibrary translates the repo's ytt library to KCL, so a data file calling one of
// its functions keeps calling it instead of freezing what it returned. A function two library
// files export under one name is dropped from both: the translation is one KCL package, which
// cannot hold the name twice.
func (m *migrator) translateYttLibrary() {
	m.libs = map[string]*yttLib{}
	if m.g.YttLibraryDirName == "" {
		return
	}
	m.libPackage = packagePath(m.g.YttLibraryDirName)
	files, err := filepath.Glob(filepath.Join(m.g.RootDir, m.g.YttLibraryDirName, "*.star"))
	if err != nil {
		return
	}
	exported := map[string]string{}
	for _, file := range files {
		content, err := os.ReadFile(file) // #nosec G304 -- paths come from globbing the repo being migrated
		if err != nil {
			continue
		}
		lib := translateYttLib(file, content)
		if lib == nil {
			continue
		}
		for name := range lib.funcs {
			if owner, taken := exported[name]; taken {
				log.Debug().Msg(m.g.Msg(fmt.Sprintf("%s and %s both define %s; neither is translated", owner, file, name)))
				delete(m.libs, filepath.Base(owner))
				lib = nil
				break
			}
			exported[name] = file
		}
		if lib != nil {
			m.libs[lib.name] = lib
		}
	}
}

// emittedLibs lists the translated library files to write, sorted. The translation is one KCL
// package, so it is written whole as soon as anything imports it.
func (m *migrator) emittedLibs() []*yttLib {
	if !m.libImported {
		return nil
	}
	libs := make([]*yttLib, 0, len(m.libs))
	for _, name := range slices.Sorted(maps.Keys(m.libs)) {
		libs = append(libs, m.libs[name])
	}
	return libs
}

// deriveStandalone translates the ytt computation of a file ytt resolved standalone into KCL
// and keeps what KCL reproduces (verifyDerivations). What it does not translate is converted
// as the literal ytt resolved and reported: the file reads nothing outside itself, so the
// literal says everything the computation did, but the derivation behind it is lost.
func (m *migrator) deriveStandalone(file string, content []byte, converted *convertedFile) {
	derived := yttDerivations(file, content, m.libs, m.libPackage, "")
	m.verifyDerivations(file, derived, converted.values)
	if derived.has() {
		converted.derived = derived
		m.derivedCount += len(derived.exprs)
		m.libImported = m.libImported || len(derived.imports) > 0
	}

	split, err := splitYttFile(content)
	if err != nil {
		m.resolved = append(m.resolved, skippedFile{file: file, deferred: []string{"its computed values"}})
		return
	}
	literal := slices.DeleteFunc(slices.Clone(split.deferred), func(path string) bool {
		return converted.derived.hasPath(path)
	})
	if len(literal) > 0 {
		m.resolved = append(m.resolved, skippedFile{file: file, deferred: literal})
	}
}

// resolveDataValues resolves one plain data-values document the way ytt does, on its own: the
// repo's ytt library is on the path, nothing else, so the result is the file's own values with
// its Starlark evaluated — no schema defaults from elsewhere mixed in.
func (m *migrator) resolveDataValues(file string, content []byte) (map[string]any, error) {
	paths, err := m.standalonePaths(file, content)
	if err != nil {
		return nil, err
	}
	res, err := runYttWithFilesAndStdin("migrate", paths, nil, func(name string, err error, stderr string, args []string) {
		if err != nil {
			log.Debug().Str("stderr", stderr).Msg(m.g.Msg(msgRunCmd("inspect data values", name, args)))
		}
	}, "--data-values-inspect")
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", file, err)
	}
	values := map[string]any{}
	if err := yaml.Unmarshal([]byte(res.Stdout), &values); err != nil {
		return nil, fmt.Errorf("parsing the resolved values of %s: %w", file, err)
	}
	return values, nil
}

// standalonePaths is the ytt file set that resolves one data file by itself: the repo's ytt
// library directory, plus a copy of the content to resolve. A ytt load resolves against the
// file set rather than the filesystem, so the copy may live anywhere; its name only has to be
// unique and keep the .yaml extension.
func (m *migrator) standalonePaths(file string, content []byte) ([]string, error) {
	paths := []string{}
	if m.g.YttLibraryDirName != "" {
		libDir := filepath.Join(m.g.RootDir, m.g.YttLibraryDirName)
		if ok, err := isExist(libDir); err == nil && ok {
			paths = append(paths, libDir)
		}
	}
	flat := nonIdentifierCharRe.ReplaceAllString(strings.TrimSuffix(file, filepath.Ext(file)), "_")
	copied := filepath.Join(m.g.RootDir, m.g.ServiceDirName, m.g.TempDirName, "migrate", "standalone", flat+".yaml")
	if err := writeFile(copied, content); err != nil {
		return nil, fmt.Errorf("writing the standalone copy of %s: %w", file, err)
	}
	return append(paths, copied), nil
}

// inspectSchema resolves one schema document the way ytt sees it. Only the schema is
// inspected, so a schema whose defaults would fail its own validations (`min_len=1` on an
// empty default, which a prototype uses to demand a value) still converts. content is what is
// inspected — the file itself, or what splitYttFile left of it.
//
// The repo's ytt library directory is on the path, so a schema loading a repo-local Starlark
// helper resolves like it does at render time. The engine's own data schema is not: it would
// merge its defaults into this document's.
func (m *migrator) inspectSchema(file string, content []byte) (*inspectedSchema, error) {
	paths, err := m.standalonePaths(file, content)
	if err != nil {
		return nil, err
	}
	res, err := runYttWithFilesAndStdin("migrate", paths, nil, func(name string, err error, stderr string, args []string) {
		if err != nil {
			log.Debug().Str("stderr", stderr).Msg(m.g.Msg(msgRunCmd("inspect data values schema", name, args)))
		}
	}, "--data-values-schema-inspect", "--output=openapi-v3")
	if err != nil {
		return nil, fmt.Errorf("inspecting the schema of %s: %w", file, err)
	}
	return parseSchemaInspect([]byte(res.Stdout))
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
	moveKey(m.protoDerived, oldName, newName)
	moveKey(m.protoComments, oldName, newName)
	moveKey(m.protoInspected, oldName, newName)
	moveKey(m.protoSchemaOutside, oldName, newName)
	for _, node := range m.nodes {
		moveKey(node.protoValues, oldName, newName)
		moveKey(node.protoDerived, oldName, newName)
		moveKey(node.protoComments, oldName, newName)
		moveKey(node.protoContextual, oldName, newName)
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
	m.protoPlans = map[string]*protoSchemaPlan{}
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
		var demanded map[string]bool
		if schema := m.protoInspected[proto]; schema != nil {
			demanded = schema.demanded
		}
		// A prototype whose only values are demanded ones (pruned away, see
		// pruneDemandedDefaults) still needs a schema: that is where their checks live.
		if len(values) == 0 && len(demanded) == 0 {
			continue
		}
		if !isKclIdentifier(proto) || kclGeneratedNames[proto] {
			m.warn("%s/%s: no base schema generated (the converter could not derive a KCL package name for this directory); its defaults are repeated in every application declaration instead — rename it by hand to an identifier that starts with a letter or underscore and is neither a KCL keyword nor one of %q, then migrate again",
				m.g.PrototypesDir, proto, slices.Sorted(maps.Keys(kclGeneratedNames)))
			continue
		}
		var unusable []string
		attributes := slices.Collect(maps.Keys(values))
		for key := range demanded {
			// Only a top-level demanded value becomes an attribute of the root schema.
			if !strings.Contains(key, "\x00") {
				attributes = append(attributes, key)
			}
		}
		slices.Sort(attributes)
		for _, key := range slices.Compact(attributes) {
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
		plan := newProtoSchemaPlan(m.protoSchemas[proto], values, m.protoInspected[proto])
		plan.externalSchema = m.protoSchemaOutside[proto]
		m.protoPlans[proto] = plan
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

// declarationComments merges the comments a declaration carries, in the same order as its
// values. With a generated base schema the prototype's own comments stay in proto.k next to
// the defaults they belong to; without one they are hoisted along with those defaults.
func (m *migrator) declarationComments(decl *migNode, name, proto string) map[string][]string {
	var parts []map[string][]string
	if m.protoSchemas[proto] == "" {
		parts = append(parts, m.protoComments[proto])
	}
	for _, node := range decl.chain() {
		parts = append(parts, node.protoComments[proto])
	}
	for _, node := range decl.chain() {
		parts = append(parts, node.appComments[name])
	}
	return mergeComments(parts...)
}

// declarationDerived collects the KCL translations behind the values a declaration carries,
// mirroring declarationValues source for source.
func (m *migrator) declarationDerived(decl *migNode, name, proto string) *derivations {
	var parts []*derivations
	if m.protoSchemas[proto] == "" {
		parts = append(parts, m.protoDerived[proto])
	}
	for _, node := range decl.chain() {
		parts = append(parts, node.protoDerived[proto])
	}
	for _, node := range decl.chain() {
		parts = append(parts, node.appDerived[name])
	}
	return mergeDerivations(parts...)
}

// appContextual collects the candidate translations of what an application's level files
// compute from the data values of their environment.
func appContextual(chain []*migNode, name, proto string) *derivations {
	var parts []*derivations
	for _, node := range chain {
		parts = append(parts, node.protoContextual[proto], node.appContextual[name])
	}
	return mergeDerivations(parts...)
}

// protoOf names the prototype an application runs at or below a level. An override level has
// no roster of its own, so the answer comes from the environments underneath it.
func protoOf(node *migNode, name string) string {
	for _, leaf := range node.leaves {
		if proto, ok := leaf.env.foundApplications[name]; ok {
			return proto
		}
	}
	return ""
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
		leaf.envPatch = m.diffValues(leafDir, simEnv, legacyEnv, nil)
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
						treeApp = m.protoPlans[decl.proto].withElementDefaults(
							mergeValues(m.protoBase[decl.proto], decl.values), nil)
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
			patch := m.diffValues(leafDir+"/"+app.Name, simApp, legacyApp,
				m.protoPlans[leaf.env.foundApplications[app.Name]])
			if len(patch) > 0 {
				candidates := appContextual(chain, app.Name, leaf.env.foundApplications[app.Name])
				if proven := m.proveContextual(leafDir, app.Name, candidates, patch, legacyEnv); proven.has() {
					leaf.appPatchDerived[app.Name] = proven
					m.patched -= len(proven.exprs)
					m.derivedCount += len(proven.exprs)
				}
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
// engine-owned environment scope. plan is the prototype's generated schemas, which the frozen
// arrays are pruned against; it is nil for the environment scope, which has none. Keys present in got but absent from want cannot be
// removed by merging and are reported as warnings.
func (m *migrator) diffValues(context string, got, want map[string]any, plan *protoSchemaPlan) map[string]any {
	got = withoutEngineEnvKeys(got)
	want = withoutEngineEnvKeys(want)

	var extra, lists []string
	patch := diffValueMaps(got, want, "", &extra, &lists)
	// A frozen array is stated whole, but its elements are instantiated by the schema that
	// types them, so the literal drops what that schema already supplies.
	plan.pruneElementDefaults(patch, nil)
	for _, path := range extra {
		m.warn("%s: converted value %s is absent from the legacy-resolved output and cannot be removed by merging; drop it by hand", context, path)
	}
	for _, path := range lists {
		// Reported only if it is still a literal once the derivations are proven.
		m.frozenLists = append(m.frozenLists, frozenList{context: context, path: path})
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

// stillFrozen reports whether any leaf still states this value path as a literal, rather than
// as the derivation the converter found for it. A value whose every leaf is a derivation is
// no longer frozen, even where the path itself carries none — an array states its elements.
func (m *migrator) stillFrozen(path string) bool {
	for _, node := range m.nodes {
		if value, found := valueAtPath(node.envPatch, path); found && patchHasLiterals(value, nil, path) {
			return true
		}
		for name, patch := range node.appPatches {
			if value, found := valueAtPath(patch, path); found && patchHasLiterals(value, node.appPatchDerived[name], path) {
				return true
			}
		}
	}
	return false
}

// warnNow prints one warning the report decided to keep.
func (m *migrator) warnNow(format string, args ...any) {
	log.Warn().Msg(m.g.Msg(fmt.Sprintf(format, args...)))
}

func (m *migrator) warn(format string, args ...any) {
	m.warnings = append(m.warnings, fmt.Sprintf(format, args...))
}

func (m *migrator) printReport() {
	for _, skipped := range m.skipped {
		if len(skipped.deferred) == 0 {
			log.Warn().Msg(m.g.Msg(fmt.Sprintf(
				"Skipped %s: it contains ytt logic; its resolved values are frozen in leaf-level TODO patches", skipped.file)))
			continue
		}
		frozen := slices.DeleteFunc(slices.Clone(skipped.deferred), func(path string) bool { return !m.stillFrozen(path) })
		if len(frozen) == 0 {
			// Every value taken out of the file reached the generated tree as a derivation.
			continue
		}
		log.Warn().Msg(m.g.Msg(fmt.Sprintf(
			"Converted %s without %s: computed by ytt logic, frozen in leaf-level TODO patches",
			skipped.file, strings.Join(frozen, ", "))))
	}
	for _, warning := range m.warnings {
		log.Warn().Msg(m.g.Msg(warning))
	}
	for _, frozen := range m.frozenLists {
		if !m.stillFrozen(frozen.path) {
			continue
		}
		m.warnNow("%s: array value %s is frozen in a patch, but ytt appends arrays over schema defaults; if the gate reports a difference here, fix it by hand", frozen.context, frozen.path)
	}
	for _, resolved := range m.resolved {
		// Such a file reads nothing outside itself, so the literal ytt resolved says
		// everything its computation did: this is a note about readability, not a task.
		log.Info().Msg(m.g.Msg(fmt.Sprintf(
			"Resolved %s with ytt: %s are converted as literals, their ytt logic having no KCL translation",
			resolved.file, strings.Join(resolved.deferred, ", "))))
	}
	if m.derivedCount > 0 {
		log.Info().Msg(m.g.Msg(fmt.Sprintf(
			"Translated %d ytt-computed value(s) into KCL derivations", m.derivedCount)))
	}
	if m.patched > 0 {
		log.Info().Msg(m.g.Msg(fmt.Sprintf(
			"Froze %d resolved value(s) in leaf-level TODO patches; turn them into KCL derivations (see docs/migration.md)", m.patched)))
	}
	log.Info().Msg(m.g.Msg(
		"Migration seed complete. Verify with the byte-identical gate (render and diff rendered/), " +
			"then hand-finish following docs/migration.md"))
}
