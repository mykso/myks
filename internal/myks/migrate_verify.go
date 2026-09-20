package myks

import (
	"bytes"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
	kcl "kcl-lang.io/kcl-go"
)

// A translation is only carried into the generated tree once KCL is shown to produce from it
// exactly what ytt resolved. Each file's derivations are evaluated in a throwaway KCL package
// — its prelude, its imports of the translated ytt library, and one variable per derived
// value — and compared against the legacy-resolved values. Whatever differs, or does not
// evaluate at all, is dropped and written as the literal instead.

// verifyDerivations drops every derivation KCL does not evaluate to the value ytt resolved.
func (m *migrator) verifyDerivations(file string, d *derivations, values map[string]any) {
	if !d.has() {
		return
	}
	paths := slices.Sorted(maps.Keys(d.exprs))
	got, err := m.evalDerivations(file, d, paths)
	if err != nil {
		log.Debug().Err(err).Msg(m.g.Msg("Evaluating the KCL derivations of " + file))
		clear(d.exprs)
		return
	}
	for i, path := range paths {
		want, found := valueAtPath(values, path)
		if !found || !sameValue(got[verifiedVar(i)], want) {
			log.Debug().Msg(m.g.Msg(fmt.Sprintf("KCL derivation of %s in %s does not reproduce the resolved value", path, file)))
			delete(d.exprs, path)
		}
	}
	d.prelude = prunePrelude(d.prelude, d.exprs)
}

// proveContextual keeps the candidate derivations of one application that KCL evaluates, at
// this leaf, to exactly the value frozen for it here. The level variable is bound to the
// leaf's resolved environment data — what `@myks:data.lib.yaml` handed the legacy file — so a
// derivation survives only where reading the environment reproduces what ytt computed from
// it. At a leaf where it does not, the literal stays frozen.
func (m *migrator) proveContextual(leafDir, unit string, candidates *derivations, frozen, levelValues map[string]any) *derivations {
	if !candidates.has() {
		return nil
	}
	var paths []string
	for _, path := range slices.Sorted(maps.Keys(candidates.exprs)) {
		if _, found := valueAtPath(frozen, path); found {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return nil
	}
	level, err := kclLiteral(levelValues)
	if err != nil {
		return nil
	}

	unitPath := filepath.Join(leafDir, unit)
	trial := &derivations{
		exprs:   candidates.exprs,
		prelude: append([]string{levelVarName + " = " + level}, candidates.prelude...),
		imports: candidates.imports,
	}
	got, err := m.evalDerivations(unitPath, trial, paths)
	if err != nil {
		log.Debug().Err(err).Msg(m.g.Msg("Evaluating the environment-derived values of " + unitPath))
		return nil
	}

	proven := &derivations{exprs: map[string]string{}}
	for i, path := range paths {
		want, _ := valueAtPath(frozen, path)
		if sameValue(got[verifiedVar(i)], want) {
			proven.exprs[path] = candidates.exprs[path]
		}
	}
	if len(proven.exprs) == 0 {
		return nil
	}
	proven.prelude = prunePrelude(candidates.prelude, proven.exprs)
	proven.imports = usedImports(candidates.imports, proven)
	return proven
}

// usedImports keeps the imports the surviving statements still read.
func usedImports(imports []string, d *derivations) []string {
	var kept []string
	for _, statement := range imports {
		pkg := statement[strings.LastIndex(statement, " ")+1:]
		reads := slices.ContainsFunc(slices.Collect(maps.Values(d.exprs)), func(expr string) bool {
			return readsName(expr, pkg)
		}) || slices.ContainsFunc(d.prelude, func(stmt string) bool { return readsName(stmt, pkg) })
		if reads {
			kept = append(kept, statement)
		}
	}
	return kept
}

// evalDerivations evaluates one file's derivations in a throwaway KCL package.
func (m *migrator) evalDerivations(file string, d *derivations, paths []string) (map[string]any, error) {
	dir := filepath.Join(m.g.RootDir, m.g.ServiceDirName, m.g.TempDirName, "migrate", "verify",
		nonIdentifierCharRe.ReplaceAllString(strings.TrimSuffix(file, filepath.Ext(file)), "_"))
	if err := writeFile(filepath.Join(dir, kclModFileName), []byte("[package]\nname = \"verify\"\nversion = \"0.0.1\"\n")); err != nil {
		return nil, err
	}
	for _, lib := range m.libs {
		if err := writeFile(m.libKPath(dir, lib), []byte(lib.source)); err != nil {
			return nil, err
		}
	}

	var b strings.Builder
	for _, statement := range append(slices.Clone(d.imports), d.prelude...) {
		b.WriteString(statement + "\n")
	}
	for i, path := range paths {
		fmt.Fprintf(&b, "%s = %s\n", verifiedVar(i), d.exprs[path])
	}
	if err := writeFile(filepath.Join(dir, "main.k"), []byte(b.String())); err != nil {
		return nil, err
	}

	res, err := kcl.Run(dir)
	if err != nil {
		return nil, fmt.Errorf("%s", cleanKclDiagnostics(err))
	}
	evaluated := map[string]any{}
	if err := yaml.Unmarshal([]byte(res.GetRawYamlResult()), &evaluated); err != nil {
		return nil, fmt.Errorf("parsing the evaluated derivations of %s: %w", file, err)
	}
	return evaluated, nil
}

func verifiedVar(i int) string { return fmt.Sprintf("derived%d", i) }

// valueAtPath reads the value one path addresses: `.a.b[0].c`. A key containing a dot or a
// bracket cannot be addressed, and is reported as absent.
func valueAtPath(values map[string]any, path string) (any, bool) {
	var current any = values
	for _, token := range pathTokens(path) {
		if index, isIndex := strings.CutPrefix(token, "["); isIndex {
			list, ok := current.([]any)
			if !ok {
				return nil, false
			}
			i, err := strconv.Atoi(strings.TrimSuffix(index, "]"))
			if err != nil || i < 0 || i >= len(list) {
				return nil, false
			}
			current = list[i]
			continue
		}
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		if current, ok = mapping[token]; !ok {
			return nil, false
		}
	}
	return current, true
}

// pathTokens splits a value path into its keys and its `[i]` indexes.
func pathTokens(path string) []string {
	var tokens []string
	for _, segment := range strings.Split(strings.TrimPrefix(path, "."), ".") {
		key, rest, found := strings.Cut(segment, "[")
		tokens = append(tokens, key)
		for found {
			var index string
			index, rest, found = strings.Cut(rest, "[")
			tokens = append(tokens, "["+index)
		}
	}
	return tokens
}

// sameValue compares a value KCL evaluated with one ytt resolved. Both come from YAML, so
// re-marshaling settles the numeric and container types they may disagree on.
func sameValue(got, want any) bool {
	gotYaml, err := yaml.Marshal(got)
	if err != nil {
		return false
	}
	wantYaml, err := yaml.Marshal(want)
	if err != nil {
		return false
	}
	return bytes.Equal(gotYaml, wantYaml)
}
