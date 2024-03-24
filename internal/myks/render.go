package myks

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	var lastStepOutputFile string
	var err error
	if lastStepOutputFile, err = a.Render(yamlTemplatingTools); err != nil {
		log.Error().Str("env", a.e.Id).Err(err).Msg("Failed to render")
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
	err = os.RemoveAll(destinationDir)
	if err != nil {
		log.Warn().Err(err).Str("dir", destinationDir).Msg(a.Msg(sliceStepName, "Unable to remove destination directory"))
		return err
	}
	err = os.MkdirAll(destinationDir, os.ModePerm)
	if err != nil {
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

		var obj map[string]interface{}
		err := yaml.Unmarshal([]byte(document), &obj)
		if err != nil {
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
		if ok, err := isExist(filePath); err != nil {
			return err
		} else if ok {
			log.Warn().Str("file", filePath).Msg(a.Msg(sliceStepName, "File already exists. Consider enabling render.includeNamespace"))
		}
		err = writeFile(filePath, data.Bytes())
		if err != nil {
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
	file := a.expandTempPath(fileName)
	return file, a.writeTempFile(fileName, output)
}

func (a *Application) getDestinationDir() string {
	return filepath.Join(a.e.g.RootDir, a.e.g.RenderedEnvsDir, a.e.Id, a.Name)
}

// Generates a file name for each document using kind and name if available
func genRenderedResourceFileName(resource map[string]interface{}, includeNamespace bool) (string, error) {
	kind := "NO_KIND"
	if g, ok := resource["kind"]; ok {
		kind = g.(string)
	}
	name := "NO_NAME"
	namespace := ""
	if n, ok := resource["metadata"]; ok {
		metadata := n.(map[string]interface{})
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

func (a *Application) getVendirConfigDir() (string, error) {
	resourceDir := a.expandServicePath("")
	if ok, err := isExist(resourceDir); err != nil {
		return "", err
	} else if ok {
		return resourceDir, nil
	} else {
		return "", nil
	}
}

// prepareValuesFile generates values.yaml file from ytt data files and ytt templates
// from the `helm` or `ytt` directories of the prototype and the application.
func (a *Application) prepareValuesFile(dirName string, resourceName string) (string, error) {
	valuesFileName := filepath.Join(dirName, resourceName+".yaml")

	var valuesFiles []string

	// add values file from base dir
	prototypeValuesFile := filepath.Join(a.Prototype, valuesFileName)
	if ok, err := isExist(prototypeValuesFile); err != nil {
		return "", err
	} else if ok {
		valuesFiles = append(valuesFiles, prototypeValuesFile)
	}

	// add prototype overwrites value file from env dir groups
	valuesFiles = append(valuesFiles, a.e.collectBySubpath(filepath.Join(a.e.g.PrototypeOverrideDir, a.prototypeDirName(), valuesFileName))...)

	// add application values file from env dir and groups
	valuesFiles = append(valuesFiles, a.e.collectBySubpath(filepath.Join(a.e.g.AppsDir, a.Name, valuesFileName))...)

	if len(valuesFiles) == 0 {
		log.Debug().Str("resource", resourceName).Msg(a.Msg(renderStepName, "No values files found"))
		return "", nil
	}

	resourceValuesYaml, err := a.ytt(renderStepName, "collect data values file", concatenate(a.yttDataFiles, valuesFiles))
	if err != nil {
		return "", err
	}

	if resourceValuesYaml.Stdout == "" {
		log.Warn().Msg(a.Msg(renderStepName, "Empty resource values"))
		return "", nil
	}

	err = a.writeTempFile(valuesFileName, resourceValuesYaml.Stdout)
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(renderStepName, "Unable to write resource values file"))
		return "", err
	}

	resourceValues, err := a.mergeValuesYaml(renderStepName, a.expandTempPath(valuesFileName))
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
