package myks

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
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
}

// splitYttFile removes from content every value ytt computes, together with the Starlark that
// computes it, and returns what remains. An error means the file cannot be split — the caller
// falls back to skipping it whole.
func splitYttFile(content []byte) (*yttSplit, error) {
	s := &yttSplit{lines: strings.Split(string(content), "\n")}
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
			s.drop(childPath, start, end)
			continue
		}
		switch {
		case value.Kind == yaml.MappingNode && len(value.Content) > 0:
			if s.pruneMapping(value, childPath) == 0 {
				s.drop(childPath, start, end)
				continue
			}
		default:
			// A sequence and a block scalar are converted whole, so any computation inside
			// them takes the entry with it.
			if s.computesIn(start, end) {
				s.drop(childPath, start, end)
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

func (s *yttSplit) drop(path []string, start, end int) {
	for i := start; i <= end && i < len(s.dropped); i++ {
		s.dropped[i] = true
	}
	s.deferred = append(s.deferred, "."+strings.Join(path, "."))
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
// the path of the value it annotates. A validation inside a sequence constrains one element
// rather than a path in the document and is skipped, matching the inspected schema.
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
		if value.Kind == yaml.MappingNode {
			collectValidations(value, lines, childPath, out)
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
