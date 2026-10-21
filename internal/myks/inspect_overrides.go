package myks

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

// InspectValueSource names the data-values file that last changed a final data value.
type InspectValueSource struct {
	Path string `json:"path"`
	// File is empty when no data-values file changed the value: it comes from the defaults.
	File string `json:"file"`
}

// InspectRedundantOverride is a data value that a file sets to the value it already inherits.
type InspectRedundantOverride struct {
	Path  string `json:"path"`
	File  string `json:"file"`
	Value any    `json:"value"`
}

// inspectOverrides resolves the application's data values one data-values file at a time and
// reports where each final value comes from and which values a file sets without changing them.
// Each level is rendered by ytt cumulatively, so overlays are judged by what ytt computes rather
// than by a reimplementation of their semantics.
func (a *Application) inspectOverrides() ([]InspectValueSource, []InspectRedundantOverride, error) {
	if a.e.kclMode {
		return nil, nil, errors.New("value provenance and redundant overrides are not supported in KCL mode")
	}

	base := slices.Clone(a.e.extraYttPaths)
	var dataFiles []string
	for _, f := range a.yttDataFiles {
		info, err := os.Stat(f)
		if err != nil {
			return nil, nil, fmt.Errorf("checking data file %s: %w", f, err)
		}
		// Library directories are not data values; they only need to be loadable at every level.
		if info.IsDir() {
			base = append(base, f)
		} else {
			dataFiles = append(dataFiles, f)
		}
	}
	// The sync stage adds the data values found among its vendir templates, which is where a
	// chart version is typically pinned per environment.
	vendirDirs, err := a.vendirSourceFiles()
	if err != nil {
		return nil, nil, err
	}
	for _, dir := range vendirDirs {
		files, err := dataValuesFilesIn(dir)
		if err != nil {
			return nil, nil, fmt.Errorf("collecting data values files in %s: %w", dir, err)
		}
		dataFiles = append(dataFiles, files...)
	}

	var layers []string
	var setPaths [][][]string
	for _, f := range dataFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("reading data file %s: %w", f, err)
		}
		// ytt applies every schema before any data values, whatever the file order, so schemas
		// are defaults rather than levels.
		if schemaDocRe.Match(content) {
			base = append(base, f)
			continue
		}
		paths, err := dataFileSetPaths(content)
		if err != nil {
			log.Warn().Err(err).Str("file", f).Msg(a.Msg("inspect", "Unable to read data value keys, skipping redundancy check for the file"))
		}
		layers = append(layers, f)
		setPaths = append(setPaths, paths)
	}

	values := make([]any, 0, len(layers)+1)
	for i := 0; i <= len(layers); i++ {
		dataYaml, err := a.renderDataYaml(concatenate(base, layers[:i]))
		if err != nil {
			return nil, nil, fmt.Errorf("rendering data values up to level %d: %w", i, err)
		}
		var v any
		if err := yaml.Unmarshal(dataYaml, &v); err != nil {
			return nil, nil, fmt.Errorf("parsing data values up to level %d: %w", i, err)
		}
		values = append(values, v)
	}

	sources, redundant := resolveOverrides(layers, values, setPaths)
	return sources, redundant, nil
}

// resolveOverrides compares cumulative data values: values[0] holds the defaults and
// values[i] the data values after layers[i-1] is applied; setPaths[i] lists the keys layers[i] sets.
func resolveOverrides(layers []string, values []any, setPaths [][][]string) ([]InspectValueSource, []InspectRedundantOverride) {
	var sources []InspectValueSource
	for _, path := range leafPaths(values[len(values)-1], nil) {
		source := InspectValueSource{Path: strings.Join(path, ".")}
		for i := len(layers); i > 0; i-- {
			cur, curOk := lookupPath(values[i], path)
			prev, prevOk := lookupPath(values[i-1], path)
			if curOk != prevOk || !reflect.DeepEqual(cur, prev) {
				source.File = layers[i-1]
				break
			}
		}
		sources = append(sources, source)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Path < sources[j].Path })

	var redundant []InspectRedundantOverride
	for i, paths := range setPaths {
		for _, path := range paths {
			prev, prevOk := lookupPath(values[i], path)
			cur, curOk := lookupPath(values[i+1], path)
			if prevOk && curOk && reflect.DeepEqual(cur, prev) {
				redundant = append(redundant, InspectRedundantOverride{Path: strings.Join(path, "."), File: layers[i], Value: cur})
			}
		}
	}
	return sources, redundant
}

// leafPaths lists the paths to every non-mapping value in v. Sequences are leaves: their items
// have no key to trace.
func leafPaths(v any, prefix []string) [][]string {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		if prefix == nil {
			return nil
		}
		return [][]string{prefix}
	}
	var paths [][]string
	for k, child := range m {
		paths = append(paths, leafPaths(child, slices.Concat(prefix, []string{k}))...)
	}
	return paths
}

func lookupPath(v any, path []string) (any, bool) {
	for _, k := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		if v, ok = m[k]; !ok {
			return nil, false
		}
	}
	return v, true
}

// dataValuesFileRe detects a file holding data values or a data-values schema.
var dataValuesFileRe = regexp.MustCompile(`(?m)^#@data/values\b`)

// dataValuesFilesIn returns the data-values files under root in lexical order, the order ytt
// reads a directory in.
func dataValuesFilesIn(root string) ([]string, error) {
	var files []string
	fsys := os.DirFS(root)
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if ext := filepath.Ext(path); ext != ".yaml" && ext != ".yml" {
			return nil
		}
		content, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		if dataValuesFileRe.Match(content) {
			files = append(files, filepath.Join(root, path))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", root, err)
	}
	return files, nil
}

// dataFileSetPaths returns the leaf keys a data-values file sets, read statically: ytt
// annotations are YAML comments, so a computed value still shows its key. Keys a Starlark
// loop generates are not visible and never reported.
func dataFileSetPaths(content []byte) ([][]string, error) {
	var paths [][]string
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
		paths = append(paths, mappingLeafPaths(doc.Content[0], nil)...)
	}
	return paths, nil
}

func mappingLeafPaths(node *yaml.Node, prefix []string) [][]string {
	var paths [][]string
	for i := 0; i+1 < len(node.Content); i += 2 {
		path := slices.Concat(prefix, []string{node.Content[i].Value})
		if value := node.Content[i+1]; value.Kind == yaml.MappingNode && len(value.Content) > 0 {
			paths = append(paths, mappingLeafPaths(value, path)...)
		} else {
			paths = append(paths, path)
		}
	}
	return paths
}
