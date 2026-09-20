package myks

import (
	"bytes"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
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

// valueAtPath reads the value one dotted path addresses. A key containing a dot cannot be
// addressed, and is reported as absent.
func valueAtPath(values map[string]any, path string) (any, bool) {
	keys := strings.Split(strings.TrimPrefix(path, "."), ".")
	var current any = values
	for _, key := range keys {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		if current, ok = mapping[key]; !ok {
			return nil, false
		}
	}
	return current, true
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
