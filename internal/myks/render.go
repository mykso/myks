package myks

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

type YamlTemplatingTool interface {
	Render(previousStepOutputFile string) (string, error)
	Ident() string
	IsAdditive() bool
}

func (a *Application) RenderAndSlice(yamlTemplatingTools []YamlTemplatingTool) error {
	lastStepOutputFile, err := a.Render(yamlTemplatingTools)
	if err != nil {
		log.Error().Str("env", a.e.ID).Err(err).Msg("Failed to render")
		return err
	}
	err = a.runSliceFormatStore(lastStepOutputFile)
	if err != nil {
		log.Error().Err(err).Msg("Failed to slice the output yaml")
		return err
	}
	log.Info().Msg(a.Msg(renderStepName, "Completed"))
	return nil
}

func (a *Application) Render(yamlTemplatingTools []YamlTemplatingTool) (string, error) {
	log.Debug().Msg(a.Msg(renderStepName, "Starting"))
	outputYaml := ""
	lastStepOutputFile := ""
	for nr, yamlTool := range yamlTemplatingTools {
		stepOutputYaml, err := yamlTool.Render(lastStepOutputFile)
		if err != nil {
			log.Error().Err(err).Msg(a.Msg(yamlTool.Ident(), "Failed during render step: "+yamlTool.Ident()))
			return "", err
		}
		if yamlTool.IsAdditive() {
			outputYaml = outputYaml + "\n---\n" + stepOutputYaml
		} else {
			outputYaml = stepOutputYaml
		}
		lastStepOutputFile, err = a.storeStepResult(outputYaml, yamlTool.Ident(), nr)
		if err != nil {
			log.Error().Err(err).Msg(a.Msg(yamlTool.Ident(), "Failed to store step result for: "+yamlTool.Ident()))
			return "", err
		}
	}
	return lastStepOutputFile, nil
}

func (a *Application) runSliceFormatStore(previousStepFile string) error {
	log.Debug().Msg(a.Msg(renderStepName, "Slicing"))
	data, err := os.ReadFile(filepath.Clean(previousStepFile))
	if err != nil {
		log.Warn().Err(err).Str("file", previousStepFile).Msg(a.Msg(sliceStepName, "Unable to read previous step file"))
		return err
	}

	destinationDir := a.getDestinationDir()

	// Cleanup the destination directory before writing new files
	if err = os.RemoveAll(destinationDir); err != nil {
		log.Warn().Err(err).Str("dir", destinationDir).Msg(a.Msg(sliceStepName, "Unable to remove destination directory"))
		return err
	}
	if err = os.MkdirAll(destinationDir, 0o750); err != nil {
		log.Warn().Err(err).Str("dir", destinationDir).Msg(a.Msg(sliceStepName, "Unable to create destination directory"))
		return err
	}

	// Split the document into individual YAML documents
	rgx := regexp.MustCompile(`(?m)^---\n`)
	documents := rgx.Split(string(data), -1)

	for _, document := range documents {
		if document == "" {
			continue
		}

		var obj map[string]any
		if err = yaml.Unmarshal([]byte(document), &obj); err != nil {
			log.Warn().Err(err).Str("file", previousStepFile).Msg(a.Msg(sliceStepName, "Unable to unmarshal yaml"))
			return err
		}

		var data bytes.Buffer
		enc := yaml.NewEncoder(&data)
		enc.SetIndent(2)
		err = enc.Encode(obj)
		if err != nil {
			log.Warn().Err(err).Str("file", previousStepFile).Msg(a.Msg(sliceStepName, "Unable to marshal yaml"))
			return err
		}

		fileName, err := genRenderedResourceFileName(obj, a.includeNamespace)
		if err != nil {
			log.Debug().Str("file", previousStepFile).Msg(a.Msg(sliceStepName, "File contains invalid K8s resource"))
			return err
		}
		filePath := filepath.Join(destinationDir, fileName)
		if ok, errExists := isExist(filePath); errExists != nil {
			return errExists
		} else if ok {
			log.Warn().Str("file", filePath).Msg(a.Msg(sliceStepName, "File already exists. Consider enabling render.includeNamespace"))
		}
		if err = writeFile(filePath, data.Bytes()); err != nil {
			log.Warn().Err(err).Str("file", filePath).Msg(a.Msg(renderStepName, "Unable to write file"))
			return err
		}
	}
	return nil
}

// storeStepResult saves output of a step to a file in the application's temp directory.
// Returns path to the file or an error.
func (a *Application) storeStepResult(output string, stepName string, stepNumber int) (string, error) {
	fileName := filepath.Join("steps", fmt.Sprintf("%02d-%s.yaml", stepNumber, stepName))
	file := a.expandServicePath(fileName)
	return file, a.writeServiceFile(fileName, output)
}

func (a *Application) getDestinationDir() string {
	return filepath.Join(a.e.g.RootDir, a.e.g.RenderedEnvsDir, a.e.ID, a.Name)
}

// Generates a file name for each document using kind and name if available
func genRenderedResourceFileName(resource map[string]any, includeNamespace bool) (string, error) {
	kind := "NO_KIND"
	if g, ok := resource["kind"]; ok {
		kind = g.(string)
	}
	name := "NO_NAME"
	namespace := ""
	if n, ok := resource["metadata"]; ok {
		metadata := n.(map[string]any)
		if n, ok := metadata["name"].(string); ok {
			name = n
		}
		if n, ok := metadata["namespace"].(string); ok {
			namespace = n
		}
	}
	if name == "NO_NAME" || kind == "NO_KIND" {
		return "", fmt.Errorf("invalid K8s resource encountered. No name or kind was set")
	}

	if !includeNamespace || namespace == "" {
		return fmt.Sprintf("%s-%s.yaml", strings.ToLower(kind), strings.ToLower(name)), nil
	}
	return fmt.Sprintf("%s-%s_%s.yaml", strings.ToLower(kind), strings.ToLower(name), strings.ToLower(namespace)), nil
}

// prepareValuesFile generates values.yaml file from ytt data files and ytt templates
// from the `helm` or `ytt` directories of the prototype and the application.
func (a *Application) prepareValuesFile(dirName, chartName string) (string, error) {
	var valuesFiles []string

	yttArgs := []string{"-v", "myks.context.helm.chart=" + chartName}

	if files, err := a.collectValuesFiles(dirName, "_global"); err == nil {
		valuesFiles = files
	} else {
		return "", err
	}

	if files, err := a.collectValuesFiles(dirName, chartName); err == nil {
		valuesFiles = append(valuesFiles, files...)
	} else {
		return "", err
	}

	if len(valuesFiles) == 0 {
		log.Debug().Str("resource", chartName).Msg(a.Msg(renderStepName, "No values files found"))
		return "", nil
	}

	// render zero or more yaml documents - values files
	resourceValuesYaml, err := a.ytt(renderStepName, "collect data values file", concatenate(a.yttDataFiles, valuesFiles), yttArgs...)
	if err != nil {
		return "", err
	}

	if resourceValuesYaml.Stdout == "" {
		log.Warn().Msg(a.Msg(renderStepName, "Empty resource values"))
		return "", nil
	}

	valuesFileName := filepath.Join(dirName, chartName+".yaml")
	if err = a.writeServiceFile(valuesFileName, resourceValuesYaml.Stdout); err != nil {
		log.Warn().Err(err).Msg(a.Msg(renderStepName, "Unable to write resource values file"))
		return "", err
	}

	// merge previously rendered yaml documents into one,
	// in the way Helm would do that with all values files
	resourceValues, err := a.mergeValuesYaml(renderStepName, a.expandServicePath(valuesFileName))
	if err != nil {
		return "", err
	}

	if resourceValues.Stdout == "" {
		log.Warn().Msg(a.Msg(renderStepName, "Empty resource values"))
		return "", nil
	}

	err = a.writeServiceFile(valuesFileName, resourceValues.Stdout)
	return a.expandServicePath(valuesFileName), err
}

func (a *Application) collectValuesFiles(dirName, resourceName string) ([]string, error) {
	var valuesFiles []string

	valuesFileName := filepath.Join(dirName, resourceName+".yaml")

	// add values file from base dir
	prototypeValuesFile := filepath.Join(a.Prototype, valuesFileName)
	if ok, err := isExist(prototypeValuesFile); err != nil {
		return nil, err
	} else if ok {
		valuesFiles = append(valuesFiles, prototypeValuesFile)
	}

	// add prototype overwrites value file from env dir groups
	valuesFiles = append(valuesFiles, a.e.collectBySubpath(filepath.Join(a.e.g.PrototypeOverrideDir, a.prototypeDirName(), valuesFileName))...)

	// add application values file from env dir and groups
	valuesFiles = append(valuesFiles, a.e.collectBySubpath(filepath.Join(a.e.g.AppsDir, a.Name, valuesFileName))...)

	return valuesFiles, nil
}

func (a *Application) collectFilesByGlob(subpathPattern string) ([]string, error) {
	var files []string
	currentPath := a.e.g.RootDir
	levels := strings.SplitSeq(a.e.Dir, filepath.FromSlash("/"))
	for level := range levels {
		if level == "" {
			continue
		}
		currentPath = filepath.Join(currentPath, level)
		searchPath := filepath.Join(currentPath, subpathPattern)
		if matchedFiles, err := filepath.Glob(searchPath); err == nil {
			files = append(files, matchedFiles...)
		} else {
			return nil, err
		}
	}
	return files, nil
}

func (a *Application) collectAllFilesByGlob(pattern string) ([]string, error) {
	files, err := a.collectFilesByGlob(filepath.Join(a.e.g.AppsDir, a.Name, pattern))
	if err != nil {
		return nil, err
	}
	protoOverrideFiles, err := a.collectFilesByGlob(filepath.Join(a.e.g.PrototypeOverrideDir, a.prototypeDirName(), pattern))
	if err != nil {
		return nil, err
	}
	protoFiles, err := filepath.Glob(filepath.Join(a.Prototype, pattern))
	if err != nil {
		return nil, err
	}
	return slices.Concat(files, protoOverrideFiles, protoFiles), nil
}
