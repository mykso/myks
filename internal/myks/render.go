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
		log.Error().Str("app", a.Name).Str("env", a.e.Id).Err(err).Msg("Failed to render")
	}
	err = a.runSliceFormatStore(lastStepOutputFile)
	if err != nil {
		log.Error().Err(err).Msg("Failed to slice the output yaml")
		return err
	}
	return nil
}

func (a *Application) Render(yamlTemplatingTools []YamlTemplatingTool) (string, error) {
	outputYaml := ""
	lastStepOutputFile := ""
	for _, yamlTool := range yamlTemplatingTools {
		log.Debug().Str("app", a.Name).Str("env", a.e.Id).Msg("Rendering: " + yamlTool.Ident())
		stepOutputYaml, err := yamlTool.Render(lastStepOutputFile)
		if err != nil {
			log.Error().Err(err).Msg("Failed during render step: " + yamlTool.Ident())
		}
		if yamlTool.IsAdditive() {
			outputYaml = outputYaml + "\n---\n" + stepOutputYaml
		} else {
			outputYaml = stepOutputYaml
		}
		lastStepOutputFile, err = a.storeStepResult(outputYaml, yamlTool.Ident(), 1)
		if err != nil {
			log.Error().Str("app", a.Name).Err(err).Msg("Failed to store step result for: " + yamlTool.Ident())
			return "", err
		}
	}
	return lastStepOutputFile, nil
}

func (a *Application) runSliceFormatStore(previousStepFile string) error {
	data, err := os.ReadFile(filepath.Clean(previousStepFile))
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Str("file", previousStepFile).Msg("Unable to read previous step file")
		return err
	}

	destinationDir := filepath.Join(a.e.g.RootDir, a.e.g.RenderedDir, "envs", a.e.Id, a.Name)

	// Cleanup the destination directory before writing new files
	err = os.RemoveAll(destinationDir)
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Str("dir", destinationDir).Msg("Unable to remove destination directory")
		return err
	}
	err = os.MkdirAll(destinationDir, os.ModePerm)
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Str("dir", destinationDir).Msg("Unable to create destination directory")
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
			log.Warn().Err(err).Str("app", a.Name).Str("file", previousStepFile).Msg("Unable to unmarshal yaml")
			return err
		}

		var data bytes.Buffer
		enc := yaml.NewEncoder(&data)
		enc.SetIndent(2)
		err = enc.Encode(obj)
		if err != nil {
			log.Warn().Err(err).Str("app", a.Name).Str("file", previousStepFile).Msg("Unable to marshal yaml")
			return err
		}

		fileName := genRenderedResourceFileName(obj)
		filePath := filepath.Join(destinationDir, fileName)
		// FIXME: If a file already exists, we should merge the two documents (probably).
		//        For now, we just overwrite the file and log a warning.
		if _, err := os.Stat(filePath); err == nil {
			log.Warn().Str("app", a.Name).Str("file", filePath).Msg("File already exists, check duplicated resources")
		}
		err = writeFile(filePath, data.Bytes())
		if err != nil {
			log.Warn().Err(err).Str("app", a.Name).Str("file", filePath).Msg("Unable to write file")
			return err
		}
	}
	return nil
}

// storeStepResult saves output of a step to a file in the application's temp directory.
// Returns path to the file or an error.
func (a *Application) storeStepResult(output string, stepName string, stepNumber uint) (string, error) {
	fileName := filepath.Join("steps", fmt.Sprintf("%02d-%s.yaml", stepNumber, stepName))
	file := a.expandTempPath(fileName)
	return file, a.writeTempFile(fileName, output)
}

// Generates a file name for each document using kind and name if available
func genRenderedResourceFileName(resource map[string]interface{}) string {
	kind := "NO_KIND"
	if g, ok := resource["kind"]; ok {
		kind = g.(string)
	}
	name := "NO_NAME"
	if n, ok := resource["metadata"]; ok {
		metadata := n.(map[string]interface{})
		name = metadata["name"].(string)
	}
	return fmt.Sprintf("%s-%s.yaml", strings.ToLower(kind), strings.ToLower(name))
}

func (a *Application) getVendoredDir(dirname string) (string, error) {
	resourceDir := a.expandPath(filepath.Join(a.e.g.VendorDirName, dirname))
	if _, err := os.Stat(resourceDir); err != nil {
		if os.IsNotExist(err) {
			log.Debug().Str("app", a.Name).Msg("Vendored directory directory does not exist: " + resourceDir)
			return "", nil
		}

		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to stat helm charts directory: " + resourceDir)
		return "", err
	}

	return resourceDir, nil
}

// prepareValuesFile generates values.yaml file from ytt data files and ytt templates
// from the `helm` or `ytt` directories of the prototype and the application.
func (a *Application) prepareValuesFile(dirName string, resourceName string) (string, error) {

	valuesFileName := filepath.Join(dirName, resourceName+".yaml")

	var valuesFiles []string

	prototypeValuesFile := filepath.Join(a.Prototype, valuesFileName)
	if _, err := os.Stat(prototypeValuesFile); err == nil {
		valuesFiles = append(valuesFiles, prototypeValuesFile)
	}

	valuesFiles = append(valuesFiles, a.e.collectBySubpath(filepath.Join("_apps", a.Name, valuesFileName))...)

	if len(valuesFiles) == 0 {
		log.Debug().Str("app", a.Name).Str("resource", resourceName).Msg("No values files found")
		return "", nil
	}

	log.Debug().Strs("files", valuesFiles).Str("app", a.Name).Str("resourceName", resourceName).Msg("Collected resource values templates")

	resourceValuesYaml, err := a.e.g.ytt(append(a.yttDataFiles, valuesFiles...))
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to render resource values templates")
		return "", err
	}

	if resourceValuesYaml.Stdout == "" {
		log.Warn().Str("app", a.Name).Msg("Empty resource values")
		return "", nil
	}

	err = a.writeTempFile(valuesFileName, resourceValuesYaml.Stdout)
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to write resource values file")
		return "", err
	}

	resourceValues, err := mergeValuesYaml(a.expandTempPath(valuesFileName))
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to render resource values")
		return "", err
	}

	if resourceValues.Stdout == "" {
		log.Warn().Str("app", a.Name).Msg("Empty resource values")
		return "", nil
	}

	err = a.writeServiceFile(valuesFileName, resourceValues.Stdout)
	if err != nil {
		return "", err
	}

	return a.expandServicePath(valuesFileName), nil
}
