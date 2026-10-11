package myks

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// A data-values file mixing plain YAML with ytt computation is split rather than skipped: the
// values ytt computes are taken out, and what remains converts like any plain file. The
// removed paths are reported, and their resolved values are frozen in leaf patches as before —
// so a file that computes one key no longer freezes the twenty plain ones next to it.

// safeYttAnnotations are the annotations whose value plain YAML parsing (for a data-values
// document) or `ytt --data-values-schema-inspect` (for a schema document) reproduces exactly.
// Every other annotation — Starlark code, an overlay that rewrites instead of merges,
// templated strings — marks the value it annotates as one only ytt can produce.
var safeYttAnnotations = map[string]bool{
	"data/values":                  true,
	"data/values-schema":           true,
	"schema/type":                  true,
	"schema/nullable":              true,
	"schema/default":               true,
	"schema/desc":                  true,
	"schema/title":                 true,
	"schema/examples":              true,
	"schema/validation":            true,
	"overlay/match-child-defaults": true,
}

// yttAnnotationRe captures the name of a ytt annotation at a `#@` marker; an empty capture is
// a bare `#@` marker, which introduces Starlark code.
var yttAnnotationRe = regexp.MustCompile(`#@([a-zA-Z][a-zA-Z0-9_/-]*)?`)

// lineComputes reports whether a source line carries ytt computation: bare `#@` code, or an
// annotation outside the safe set. A `#!` comment carries nothing and never matches.
func lineComputes(line string) bool {
	for _, match := range yttAnnotationRe.FindAllStringSubmatch(line, -1) {
		if !safeYttAnnotations[match[1]] {
			return true
		}
	}
	return false
}

// fileComputes reports whether a data-values file needs splitting before it can be converted.
func fileComputes(content []byte) bool {
	for line := range strings.SplitSeq(string(content), "\n") {
		if lineComputes(line) {
			return true
		}
	}
	return false
}

// yttSplit is one data-values file with its computed values taken out.
type yttSplit struct {
	// sanitized is the file without the computed values and without the Starlark that
	// produced them, ready to be converted like a plain file.
	sanitized []byte
	// deferred lists the dotted paths taken out, for the report.
	deferred []string
	// lines is the source, one entry per line.
	lines []string
	// computes and dropped mark, per line, ytt computation and removal from the output.
	computes, dropped []bool
	// kept counts the top-level document entries that survived.
	kept int
	// exprs maps the dotted path of a dropped entry to the ytt expression that computed it,
	// where the source states it inline (`key: #@ expr`).
	exprs map[string]string
	// literals maps the dotted path of a scalar inside a dropped sequence or block to the
	// value the source states for it plainly, next to the expressions: a frozen value equal
	// to it is what the source said, not something ytt computed.
	literals map[string]any
	// templates maps the dotted path of a dropped `#@yaml/text-templated-strings` scalar to
	// its text, split at the `(@= expr @)` it interpolates.
	templates map[string][]templatePart
}

// templatePart is one piece of a ytt text template: literal text, or an expression.
type templatePart struct {
	text   string
	isExpr bool
}

// textTemplateRe matches the annotation that makes ytt interpolate `(@= expr @)` in a string.
var textTemplateRe = regexp.MustCompile(`^\s*#@yaml/text-templated-strings\s*$`)

// parseTextTemplate splits a ytt text template at its `(@= expr @)` interpolations. Any other
// `(@ ... @)` block is code, which a KCL string cannot state, and fails the parse.
func parseTextTemplate(text string) ([]templatePart, bool) {
	var parts []templatePart
	for {
		start := strings.Index(text, "(@")
		if start < 0 {
			break
		}
		if !strings.HasPrefix(text[start:], "(@=") {
			return nil, false
		}
		end := strings.Index(text[start:], "@)")
		if end < 0 {
			return nil, false
		}
		expr := strings.TrimSpace(text[start+3 : start+end])
		if expr == "" || strings.HasPrefix(expr, "-") || strings.HasSuffix(expr, "-") {
			// The trim markers change the text around the interpolation.
			return nil, false
		}
		if start > 0 {
			parts = append(parts, templatePart{text: text[:start]})
		}
		parts = append(parts, templatePart{text: expr, isExpr: true})
		text = text[start+end+2:]
	}
	if text != "" {
		parts = append(parts, templatePart{text: text})
	}
	return parts, slices.ContainsFunc(parts, func(part templatePart) bool { return part.isExpr })
}

// recordTemplate records the text template of a dropped scalar whose only computation is the
// `#@yaml/text-templated-strings` annotation above it.
func (s *yttSplit) recordTemplate(value *yaml.Node, path string, start, keyLine int) {
	if value.Kind != yaml.ScalarNode {
		return
	}
	annotated := false
	for _, line := range s.lines[start : keyLine-1] {
		if !lineComputes(line) {
			continue
		}
		if !textTemplateRe.MatchString(line) {
			return
		}
		annotated = true
	}
	if !annotated || lineComputes(s.lines[keyLine-1]) {
		return
	}
	if parts, ok := parseTextTemplate(value.Value); ok {
		s.templates[path] = parts
	}
}

// splitYttFile removes from content every value ytt computes, together with the Starlark that
// computes it, and returns what remains. An error means the file cannot be split — the caller
// falls back to skipping it whole.
func splitYttFile(content []byte) (*yttSplit, error) {
	s := &yttSplit{lines: strings.Split(string(content), "\n"), exprs: map[string]string{}, literals: map[string]any{}, templates: map[string][]templatePart{}}
	s.computes = make([]bool, len(s.lines))
	s.dropped = make([]bool, len(s.lines))
	for i, line := range s.lines {
		s.computes[i] = lineComputes(line)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(content))
	for {
		var doc yaml.Node
		if err := decoder.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parsing: %w", err)
		}
		if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
			// Only a mapping document holds data values; anything else is left as it is.
			continue
		}
		s.kept += s.pruneMapping(doc.Content[0], nil)
	}

	var out []string
	for i, line := range s.lines {
		if !s.dropped[i] && !s.computes[i] {
			out = append(out, line)
		}
	}
	s.sanitized = []byte(strings.Join(out, "\n"))
	// The removals are line ranges, so a malformed result is possible; refuse it rather than
	// convert something the source never said.
	check := yaml.NewDecoder(bytes.NewReader(s.sanitized))
	for {
		var doc any
		if err := check.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("the file could not be split along its ytt computation: %w", err)
		}
	}
	s.deferred = deepestPaths(s.deferred)
	return s, nil
}

// pruneMapping drops the computed entries of one mapping and returns how many survived. A
// mapping whose entries are all computed is dropped with them: left behind, its key would
// convert to a null that overrides the defaults below it.
func (s *yttSplit) pruneMapping(node *yaml.Node, path []string) int {
	kept := 0
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		childPath := append(slices.Clone(path), key.Value)
		start, end := s.entryRange(key.Line)
		// The entry's own annotations reach from the comment block above it to its own line,
		// which is where `key: #@ expr` puts the computation.
		if s.computesIn(start, key.Line-1) {
			s.drop(childPath, key.Line, start, end)
			s.recordTemplate(value, "."+strings.Join(childPath, "."), start, key.Line)
			continue
		}
		switch {
		case value.Kind == yaml.MappingNode && len(value.Content) > 0:
			if s.pruneMapping(value, childPath) == 0 {
				s.drop(childPath, key.Line, start, end)
				continue
			}
		default:
			// A sequence and a block scalar are converted whole, so any computation inside
			// them takes the entry with it. What that computation states is still recorded,
			// addressed by index, so a derivation can replace the frozen element.
			if s.computesIn(start, end) {
				s.drop(childPath, key.Line, start, end)
				s.recordExprs(value, "."+strings.Join(childPath, "."))
				continue
			}
		}
		kept++
	}
	return kept
}

// deepestPaths keeps only the paths no other path extends: a mapping is dropped when its
// entries are, so the entries alone say what ytt computed.
func deepestPaths(paths []string) []string {
	slices.Sort(paths)
	paths = slices.Compact(paths)
	deepest := make([]string, 0, len(paths))
	for i, path := range paths {
		// The paths are sorted, so an extension of one follows it immediately.
		if i+1 < len(paths) && strings.HasPrefix(paths[i+1], path+".") {
			continue
		}
		deepest = append(deepest, path)
	}
	return deepest
}

func (s *yttSplit) drop(path []string, keyLine, start, end int) {
	for i := start; i <= end && i < len(s.dropped); i++ {
		s.dropped[i] = true
	}
	dotted := "." + strings.Join(path, ".")
	s.deferred = append(s.deferred, dotted)
	if expr, ok := inlineYttExpr(s.lines[keyLine-1]); ok {
		s.exprs[dotted] = expr
	}
}

// recordExprs records the ytt expressions inside a dropped value, and the scalars it states
// plainly, addressing a sequence element by its index: `.config[0].services[1].uri`. A value
// with any other computation on or above its line is neither.
func (s *yttSplit) recordExprs(node *yaml.Node, path string) {
	switch node.Kind {
	case yaml.SequenceNode:
		if loop, ok := s.forEndLoop(node); ok {
			s.exprs[path] = loop
			return
		}
		for i, item := range node.Content {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			// A mapping item's first key shares the item's line, and is checked as a key.
			if item.Kind != yaml.MappingNode {
				if start, _ := s.entryRange(item.Line); s.computesIn(start, item.Line-1) {
					continue
				}
			}
			s.recordExprs(item, itemPath)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			childPath := path + "." + key.Value
			if start, _ := s.entryRange(key.Line); s.computesIn(start, key.Line-1) {
				if expr, ok := inlineYttExpr(s.lines[key.Line-1]); ok {
					s.exprs[childPath] = expr
				}
				s.recordTemplate(value, childPath, start, key.Line)
				continue
			}
			s.recordExprs(value, childPath)
		}
	case yaml.ScalarNode:
		var value any
		if node.Decode(&value) == nil {
			s.literals[path] = value
		}
	}
}

// forEndRe matches the annotation repeating a sequence item once per element of an iterable.
var forEndRe = regexp.MustCompile(`^\s*#@\s*for/end\s+([A-Za-z_][A-Za-z0-9_]*)\s+in\s+(.+):\s*$`)

// forEndLoop states a sequence built by `#@ for/end x in xs:` over its one item as the
// Starlark list comprehension it amounts to: `[{"name": x} for x in xs]`. Only an item whose
// computation is inline expressions qualifies, and only prelude assignments may precede the
// loop annotation; anything else is left to the resolved value.
func (s *yttSplit) forEndLoop(node *yaml.Node) (string, bool) {
	if len(node.Content) != 1 {
		return "", false
	}
	item := node.Content[0]
	annotation := item.Line - 2
	if annotation < 0 {
		return "", false
	}
	loop := forEndRe.FindStringSubmatch(s.lines[annotation])
	if loop == nil {
		return "", false
	}
	start, _ := entryRange(s.lines, item.Line)
	open := 0
	for _, line := range s.lines[start:annotation] {
		if !lineComputes(line) {
			continue
		}
		statement := strings.TrimPrefix(strings.TrimLeft(line, " "), "#@")
		if open == 0 && !assignmentRe.MatchString(statement) {
			return "", false
		}
		open += bracketBalance(statement)
	}
	if open != 0 {
		return "", false
	}
	body, ok := s.starlarkOf(item, true)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("[%s for %s in %s]", body, loop[1], strings.TrimSpace(loop[2])), true
}

// starlarkOf writes a YAML value as a Starlark expression, its inline `#@ expr` values as the
// expressions they are. A value with any other computation has none. The first key of a loop's
// item shares the item's line, whose annotations the caller has checked.
func (s *yttSplit) starlarkOf(node *yaml.Node, loopItem bool) (string, bool) {
	switch node.Kind {
	case yaml.MappingNode:
		entries := make([]string, 0, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			start := key.Line - 1
			if !loopItem || i > 0 {
				start, _ = entryRange(s.lines, key.Line)
			}
			var expr string
			if s.computesIn(start, key.Line-1) {
				inline, ok := inlineYttExpr(s.lines[key.Line-1])
				if !ok || s.computesIn(start, key.Line-2) {
					return "", false
				}
				expr = inline
			} else {
				var ok bool
				if expr, ok = s.starlarkOf(value, false); !ok {
					return "", false
				}
			}
			entries = append(entries, strconv.Quote(key.Value)+": "+expr)
		}
		return "{" + strings.Join(entries, ", ") + "}", true
	case yaml.SequenceNode:
		elements := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			if start, _ := entryRange(s.lines, item.Line); s.computesIn(start, item.Line-1) {
				return "", false
			}
			element, ok := s.starlarkOf(item, false)
			if !ok {
				return "", false
			}
			elements = append(elements, element)
		}
		return "[" + strings.Join(elements, ", ") + "]", true
	case yaml.ScalarNode:
		var value any
		if node.Decode(&value) != nil {
			return "", false
		}
		return starlarkScalar(value)
	}
	return "", false
}

// starlarkScalar writes a decoded YAML scalar as a Starlark literal.
func starlarkScalar(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "None", true
	case bool:
		if typed {
			return "True", true
		}
		return "False", true
	case string:
		return strconv.Quote(typed), true
	case int:
		return strconv.Itoa(typed), true
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return "", false
		}
		formatted := strconv.FormatFloat(typed, 'g', -1, 64)
		if !strings.ContainsAny(formatted, ".e") {
			formatted += ".0"
		}
		return formatted, true
	}
	return "", false
}

// inlineYttExpr returns the ytt expression a mapping entry states on its own line, as in
// `port: #@ 8080`. An entry whose value is computed anywhere else — a templated block, an
// annotation above it — states none.
func inlineYttExpr(line string) (string, bool) {
	marker := strings.Index(line, "#@")
	if marker < 0 || !strings.HasSuffix(strings.TrimSpace(line[:marker]), ":") {
		return "", false
	}
	expr := strings.TrimSpace(line[marker+2:])
	if expr == "" {
		return "", false
	}
	return expr, true
}

func (s *yttSplit) computesIn(start, end int) bool {
	for i := start; i <= end && i < len(s.computes); i++ {
		if s.computes[i] {
			return true
		}
	}
	return false
}

func (s *yttSplit) entryRange(keyLine int) (start, end int) {
	return entryRange(s.lines, keyLine)
}

// entryRange returns the line index range (0-based, inclusive) of the mapping entry whose key
// is on line keyLine (1-based): the comment block directly above it, the key line itself, and
// everything indented under it.
func entryRange(lines []string, keyLine int) (start, end int) {
	start, end = keyLine-1, keyLine-1
	for start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), "#") {
		start--
	}
	indent := lineIndent(lines[end])
	for i := end + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		childIndent := lineIndent(lines[i])
		// A sequence may sit at its key's own indentation, and still belongs to it.
		if childIndent < indent || (childIndent == indent && !strings.HasPrefix(trimmed, "- ")) {
			break
		}
		end = i
	}
	return start, end
}

func lineIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// A `#@schema/validation` annotation is Starlark, not YAML, so ytt's OpenAPI schema output
// carries only the keyword arguments that have an OpenAPI counterpart. The rest — `not_null`,
// a custom rule — is read from the source text below, anchored at the path of the value it
// annotates, so `not_null` reaches the generated KCL check block and the others are reported
// naming their path.

// yttValidation is one `#@schema/validation` annotation of a schema document.
type yttValidation struct {
	path []string // the path of the annotated value, from the document root
	// kwargs lists the keyword argument names the annotation passes; a custom rule (a lambda
	// or a named function, which names nothing) is reported as one empty name.
	kwargs []string
}

var (
	// validationLineRe captures the arguments of a `#@schema/validation` annotation line.
	validationLineRe = regexp.MustCompile(`^\s*#@schema/validation\s+(.*)$`)
	// kwargNameRe matches a keyword argument name in that argument list.
	kwargNameRe = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\s*=`)
	// comparisonRe matches the operators of a custom rule's expression, which would otherwise
	// read as keyword arguments (`lambda v: v == v.lower()`).
	comparisonRe = regexp.MustCompile(`[!<>=]=`)
)

// yttValidations returns the `#@schema/validation` annotations of a schema document, each with
// the path of the value it annotates. A validation on a key of a sequence's element is
// anchored below the sequence at itemsKey, matching the inspected schema.
func yttValidations(content []byte) ([]yttValidation, error) {
	lines := strings.Split(string(content), "\n")
	var found []yttValidation
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	for {
		var doc yaml.Node
		if err := decoder.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parsing: %w", err)
		}
		if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
			continue
		}
		collectValidations(doc.Content[0], lines, nil, &found)
	}
	return found, nil
}

func collectValidations(node *yaml.Node, lines, path []string, out *[]yttValidation) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		childPath := append(slices.Clone(path), key.Value)
		// The annotations of an entry reach from the comment block above it to its own line.
		start, _ := entryRange(lines, key.Line)
		for _, line := range lines[start:key.Line] {
			match := validationLineRe.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			*out = append(*out, yttValidation{path: childPath, kwargs: validationKwargs(match[1])})
		}
		switch {
		case value.Kind == yaml.MappingNode:
			collectValidations(value, lines, childPath, out)
		case value.Kind == yaml.SequenceNode && len(value.Content) > 0 && value.Content[0].Kind == yaml.MappingNode:
			// A schema's one sequence item describes every element.
			collectValidations(value.Content[0], lines, append(childPath, itemsKey), out)
		}
	}
}

// validationKwargs names the keyword arguments a validation's argument list passes. A list
// that names none is a custom rule, reported as one empty name.
func validationKwargs(args string) []string {
	var names []string
	for _, match := range kwargNameRe.FindAllStringSubmatch(comparisonRe.ReplaceAllString(args, " "), -1) {
		names = append(names, match[1])
	}
	if len(names) == 0 {
		names = []string{""}
	}
	return names
}

// yttComment is the comment block a data-values file writes above one of its values, ready to
// be written into the generated KCL, or the order a mapping states its keys in (keyOrderPath).
type yttComment struct {
	path  string // the dotted value path, as the KCL writer addresses values
	lines []string
}

// yttComments returns the comments a data-values file writes above its mapping keys, each with
// the path of the value it belongs to. The conversion moves a value out of the file it was
// written in, and what someone wrote next to it — why the value is what it is, a tool's
// annotation such as Renovate's — has to travel with it. A blank line above a key travels as
// an empty line of its block, so the groups the file separated stay separated.
//
// It also returns the order every mapping states its keys in: the converted values are Go
// maps, and the generated file would otherwise list them alphabetically.
//
// Only the block above a mapping key or a sequence item is read. A comment after a value has
// no attribute of its own to sit above in the generated schema, and is left behind. In a
// schema document a sequence's one item describes every element, so what is written inside
// it belongs to the element schema; in a data-values document each item is addressed by its
// index.
func yttComments(content []byte, schema bool) ([]yttComment, error) {
	lines := strings.Split(string(content), "\n")
	var found []yttComment
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	for {
		var doc yaml.Node
		if err := decoder.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parsing: %w", err)
		}
		if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
			continue
		}
		collectComments(doc.Content[0], lines, "", schema, &found)
	}
	if trailing := trailingComments(lines); len(trailing) > 0 {
		found = append(found, yttComment{path: trailingCommentsPath, lines: trailing})
	}
	return found, nil
}

// trailingCommentsPath is where the comment index of a file keeps the block the file ends
// with: no value path is empty.
const trailingCommentsPath = ""

// keyOrderPath is where the comment index of a file keeps the key order of the mapping at a
// dotted path. The prefix is no possible start of a value path, which always starts with a dot.
func keyOrderPath(path string) string { return "\x02" + path }

// trailingComments returns the comment block a file ends with, which sits above no value. The
// generated file ends with it too, so that a note left there — a Renovate marker tracking a
// version the file does not state, say — is not lost with the legacy file.
func trailingComments(lines []string) []string {
	var block []string
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			break
		}
		if !strings.HasPrefix(line, "#@") {
			block = append([]string{kclComment(line)}, block...)
		}
	}
	return block
}

func collectComments(node *yaml.Node, lines []string, path string, schema bool, out *[]yttComment) {
	keys := make([]string, 0, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		childPath := path + "." + key.Value
		keys = append(keys, key.Value)
		collectBlock(lines, key.Line, i > 0, childPath, out)
		collectNested(value, lines, childPath, schema, out)
	}
	*out = append(*out, yttComment{path: keyOrderPath(path), lines: keys})
}

// collectNested descends into a mapping or sequence value.
func collectNested(node *yaml.Node, lines []string, path string, schema bool, out *[]yttComment) {
	switch node.Kind {
	case yaml.MappingNode:
		collectComments(node, lines, path, schema, out)
	case yaml.SequenceNode:
		for i, item := range node.Content {
			itemPath := elementPath(path, i)
			if schema {
				itemPath = path + "." + itemsKey
			}
			if item.Kind != yaml.MappingNode {
				// A mapping item's first key sits on the item's line and claims its block.
				collectBlock(lines, item.Line, i > 0, itemPath, out)
			}
			collectNested(item, lines, itemPath, schema, out)
			if schema {
				break
			}
		}
	}
}

// collectBlock records the comment block above the entry on line (1-based): from the blank
// line above it to the entry itself. A blank line above an entry that is not the first of its
// collection is recorded as an empty first line.
func collectBlock(lines []string, line int, separable bool, path string, out *[]yttComment) {
	start, _ := entryRange(lines, line)
	var block []string
	if separable && start > 0 && strings.TrimSpace(lines[start-1]) == "" {
		block = append(block, "")
	}
	for _, text := range lines[start : line-1] {
		text = strings.TrimSpace(text)
		// A `#@` directive is a comment to YAML but code to ytt: it carries no meaning
		// into KCL, where the value it annotated is stated in KCL's own terms.
		if !strings.HasPrefix(text, "#") || strings.HasPrefix(text, "#@") {
			continue
		}
		block = append(block, kclComment(text))
	}
	if len(block) > 0 {
		*out = append(*out, yttComment{path: path, lines: block})
	}
}

// kclComment rewrites one comment line as KCL writes comments. ytt's own comment marker
// (`#!`) means nothing outside a ytt template, so only the `#` of it is kept — which is what
// a pattern matching such a comment in the generated files has to look for.
func kclComment(line string) string {
	if rest, ok := strings.CutPrefix(line, "#!"); ok {
		line = "#" + rest
	}
	if rest, ok := strings.CutPrefix(line, "#"); ok && rest != "" && !strings.HasPrefix(rest, " ") {
		line = "# " + rest
	}
	return line
}
