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

	yaml "gopkg.in/yaml.v3"
)

// InspectValueSource names the file that last changed a final data value or helm or ytt-pkg value.
type InspectValueSource struct {
	Path string `json:"path"`
	// File is empty when no data-values or schema file of the application changed a data value:
	// it comes from the myks built-in defaults.
	File string `json:"file"`
}

// InspectRedundantOverride is a data value or helm or ytt-pkg value that a file sets to the value it already inherits.
type InspectRedundantOverride struct {
	Path  string `json:"path"`
	File  string `json:"file"`
	Value any    `json:"value"`
}

// inspectOverrides resolves the application's data values one schema or data-values file at a
// time and reports where each final value comes from and which values a file sets without
// changing them.
// Each level is rendered by ytt cumulatively, so overlays are judged by what ytt computes rather
// than by a reimplementation of their semantics. withVendir also checks the data values found
// among the vendir templates for redundancy.
func (a *Application) inspectOverrides(withVendir bool) ([]InspectValueSource, []InspectRedundantOverride, error) {
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
	schemas, layers, setPaths, err := splitDataFiles(dataFiles)
	if err != nil {
		return nil, nil, err
	}
	// Schemas are the first levels, so each default is traced to the schema that declares it.
	// Levels are cumulative, so a schema extending an earlier one still loads.
	levels := slices.Concat(schemas, layers)
	values, err := a.renderLevels(base, levels)
	if err != nil {
		return nil, nil, err
	}
	sources := valueSources(levels, values)
	redundant := redundantOverrides(layers, values[len(schemas):], setPaths)
	if !withVendir {
		return sources, redundant, nil
	}

	vendirDirs, err := a.vendirSourceFiles()
	if err != nil {
		return nil, nil, err
	}
	var vendirFiles []string
	for _, dir := range vendirDirs {
		files, err := dataValuesFilesIn(dir)
		if err != nil {
			return nil, nil, fmt.Errorf("collecting data values files in %s: %w", dir, err)
		}
		vendirFiles = append(vendirFiles, files...)
	}
	vendirSchemas, vendirLayers, vendirSetPaths, err := splitDataFiles(vendirFiles)
	if err != nil {
		return nil, nil, err
	}
	if len(vendirLayers) == 0 {
		return sources, redundant, nil
	}
	// Only the vendir config sees these data values, on top of the application's final ones;
	// the render steps never do, so they are no source of a final value.
	vendirValues, err := a.renderLevels(slices.Concat(base, schemas, vendirSchemas, layers), vendirLayers)
	if err != nil {
		return nil, nil, err
	}
	redundant = append(redundant, redundantOverrides(vendirLayers, vendirValues, vendirSetPaths)...)
	return sources, redundant, nil
}

// runtimeDataKey holds the data values myks detects from the checkout, e.g. the git remote.
const runtimeDataKey = "myks"

// splitDataFiles separates data-values schemas from data values and reads the keys each data
// values file sets, except runtime data. ytt applies every schema before any data values, whatever the file order.
func splitDataFiles(files []string) (schemas, layers []string, setPaths [][][]string, err error) {
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("reading data file %s: %w", f, err)
		}
		if schemaDocRe.Match(content) {
			schemas = append(schemas, f)
			continue
		}
		paths, err := dataFileSetPaths(content)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("reading data value keys of %s: %w", f, err)
		}
		// A file pins runtime data because another checkout may detect something else.
		paths = slices.DeleteFunc(paths, func(p []string) bool { return p[0] == runtimeDataKey })
		layers = append(layers, f)
		setPaths = append(setPaths, paths)
	}
	return schemas, layers, setPaths, nil
}

// renderLevels returns the data values of base alone, then of base plus each further layer.
func (a *Application) renderLevels(base, layers []string) ([]any, error) {
	values := make([]any, 0, len(layers)+1)
	for i := 0; i <= len(layers); i++ {
		// Validations hold for the final data values only; a partial level may legitimately miss a
		// required value that a later file sets.
		dataYaml, err := a.renderDataYaml(concatenate(base, layers[:i]), "--dangerous-data-values-disable-validation")
		if err != nil {
			return nil, fmt.Errorf("rendering data values up to level %d: %w", i, err)
		}
		var v any
		if err := yaml.Unmarshal(dataYaml, &v); err != nil {
			return nil, fmt.Errorf("parsing data values up to level %d: %w", i, err)
		}
		values = append(values, v)
	}
	return values, nil
}

// valueSources names, for every final value, the last layer that changed it. values[i] holds
// the data values of the built-in defaults plus layers[:i].
func valueSources(layers []string, values []any) []InspectValueSource {
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
	return sources
}

// redundantOverrides lists the keys layers[i] sets, setPaths[i], whose value is the same before
// and after it. values[i] holds the data values of the defaults plus layers[:i].
func redundantOverrides(layers []string, values []any, setPaths [][][]string) []InspectRedundantOverride {
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
	return redundant
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
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Data values of a private library apply to that library only, never to the root.
			if d.Name() == "_ytt_lib" {
				return fs.SkipDir
			}
			return nil
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

// dataFileSetPaths returns the leaf keys a data-values file sets, sorted and each once, read
// statically: ytt annotations are YAML comments, so a computed value still shows its key. Keys a
// Starlark loop generates are not visible and never reported.
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
	// Several documents of one file may set the same key; the file is judged as a whole.
	slices.SortFunc(paths, slices.Compare[[]string])
	return slices.CompactFunc(paths, slices.Equal[[]string]), nil
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

// inspectValuesOverrides resolves the values of every chart, or ytt package, of the step
// directory dirName one values file at a time, in the order prepareValuesFile applies them, and
// reports per chart where each final value comes from and which values a file sets without
// changing them. The charts are those with values files of their own plus those vendored in
// vendorDirName, which still get the global values files.
func (a *Application) inspectValuesOverrides(dirName, vendorDirName string) (sources map[string][]InspectValueSource, redundant map[string][]InspectRedundantOverride, err error) {
	names, err := a.valuesFileTargets(dirName, vendorDirName)
	if err != nil {
		return nil, nil, err
	}
	sources = map[string][]InspectValueSource{}
	redundant = map[string][]InspectRedundantOverride{}
	for _, name := range names {
		levels, err := a.valuesFiles(dirName, name)
		if err != nil {
			return nil, nil, fmt.Errorf("collecting values files of %s: %w", name, err)
		}
		setPaths := make([][][]string, len(levels))
		for i, f := range levels {
			content, err := os.ReadFile(f)
			if err != nil {
				return nil, nil, fmt.Errorf("reading values file %s: %w", f, err)
			}
			// Values files are ytt templates too, so their keys are read the same way.
			if setPaths[i], err = dataFileSetPaths(content); err != nil {
				return nil, nil, fmt.Errorf("reading value keys of %s: %w", f, err)
			}
		}
		values := []any{nil}
		for i := range levels {
			v, err := a.mergedValues(name, levels[:i+1])
			if err != nil {
				return nil, nil, fmt.Errorf("resolving values of %s up to level %d: %w", name, i+1, err)
			}
			values = append(values, v)
		}
		if s := valueSources(levels, values); len(s) > 0 {
			sources[name] = s
		}
		if r := redundantOverrides(levels, values, setPaths); len(r) > 0 {
			redundant[name] = r
		}
	}
	return sources, redundant, nil
}

// valuesFileTargets returns the sorted names of the charts vendored in vendorDirName, the ones
// render applies values files to. Before the first sync there is no vendored directory, so it
// falls back to the names the values files in dirName imply.
func (a *Application) valuesFileTargets(dirName, vendorDirName string) ([]string, error) {
	vendorDir := a.expandVendorPath(vendorDirName)
	entries, err := os.ReadDir(vendorDir)
	if os.IsNotExist(err) {
		return a.valuesFileNames(dirName)
	} else if err != nil {
		return nil, fmt.Errorf("reading %s: %w", vendorDir, err)
	}
	var names []string
	for _, entry := range entries {
		// Vendored charts are often symlinks, so the entry type alone does not tell.
		if ok, err := isDir(filepath.Join(vendorDir, entry.Name())); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("checking %s: %w", entry.Name(), err)
		} else if ok {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

// valuesFileNames returns the sorted chart names that the values files in dirName are named for.
func (a *Application) valuesFileNames(dirName string) ([]string, error) {
	files, err := a.collectAllFilesByGlob(filepath.Join(dirName, "*.*yaml"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, f := range files {
		// prepareValuesFile matches `<chart>.*yaml`, so the chart name ends at the first dot.
		if name, _, _ := strings.Cut(filepath.Base(f), "."); name != "_global" {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return slices.Compact(names), nil
}

// mergedValues renders and merges the values files of chartName the way prepareValuesFile does.
func (a *Application) mergedValues(chartName string, files []string) (any, error) {
	unmerged, err := a.renderValuesFiles(chartName, files)
	if err != nil {
		return nil, err
	}
	if unmerged.Stdout == "" {
		return nil, nil
	}
	merged, err := a.mergeValuesYaml(renderStepName, "-", strings.NewReader(unmerged.Stdout))
	if err != nil {
		return nil, err
	}
	var v any
	if err := yaml.Unmarshal([]byte(merged.Stdout), &v); err != nil {
		return nil, fmt.Errorf("parsing merged values: %w", err)
	}
	return v, nil
}
