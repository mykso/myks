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

func (a *Application) Render() error {

	log.Debug().Str("step", "init").Str("app", a.Name).Strs("files", a.yttDataFiles).Msg("Collected ytt data files")

	// Run built-in rendering steps:
	//   a. helm
	//	 b. ytt
	//   c. local yamls
	//   d. global ytt overlays

	outputYaml, err := a.runHelm()
	if err != nil {
		return err
	}

	helmStepOutputFile := ""
	if outputYaml != "" {
		helmStepOutputFile, err = a.storeStepResult(outputYaml, "helm", 1, "")
		if err != nil {
			log.Error().Err(err).Msg("Failed to store helm step result")
			return err
		}
	}

	outputYaml, err = a.runYttPkg()
	if err != nil {
		return err
	}

	yttPkgStepOutputFile := ""
	if outputYaml != "" {
		yttPkgStepOutputFile, err = a.storeStepResult(outputYaml, "ytt-pkg", 1, helmStepOutputFile)
		if err != nil {
			log.Error().Err(err).Msg("Failed to store ytt step result")
			return err
		}
	}

	outputYaml, err = a.runYtt(yttPkgStepOutputFile)
	if err != nil {
		return err
	}

	yttStepOutput, err := a.storeStepResult(outputYaml, "ytt", 2, "")
	if err != nil {
		log.Error().Err(err).Msg("Failed to store ytt step result")
		return err
	}

	outputYaml, err = a.runGlobalYtt(yttStepOutput)
	if err != nil {
		return err
	}

	globalYttStepOutputFile, err := a.storeStepResult(outputYaml, "global.ytt", 3, "")
	if err != nil {
		log.Error().Err(err).Msg("Failed to store global ytt step result")
		return err
	}

	// 3. Run custom rendering steps: TODO

	// 4. Run kube-slice and format

	err = a.runSliceFormatStore(globalYttStepOutputFile)
	if err != nil {
		return err
	}

	return nil
}

func (a *Application) runYtt(previousStepFile string) (string, error) {

	var yttFiles []string
	yttFiles = append(yttFiles, a.yttDataFiles...)

	if previousStepFile != "" {
		yttFiles = append(yttFiles, previousStepFile)
	}

	vendorYttDir := a.expandPath(filepath.Join(a.e.g.VendorDirName, a.e.g.YttStepDirName))
	if _, err := os.Stat(vendorYttDir); err == nil {
		yttFiles = append(yttFiles, vendorYttDir)
	}

	prototypeYttDir := filepath.Join(a.Prototype, a.e.g.YttStepDirName)
	if _, err := os.Stat(prototypeYttDir); err == nil {
		yttFiles = append(yttFiles, prototypeYttDir)
	}

	yttFiles = append(yttFiles, a.e.collectBySubpath(filepath.Join("_apps", a.Name, a.e.g.YttStepDirName))...)

	if len(yttFiles) == 0 {
		log.Debug().Str("app", a.Name).Msg("No yaml files found")
		return "", nil
	}

	log.Debug().Strs("files", yttFiles).Str("app", a.Name).Msg("Collected ytt files")

	yamlOutput, err := a.e.g.ytt(yttFiles)
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to render ytt files")
		return "", err
	}

	if yamlOutput.Stdout == "" {
		log.Warn().Str("app", a.Name).Msg("Empty ytt output")
		return "", nil
	}

	return yamlOutput.Stdout, nil
}

func (a *Application) runGlobalYtt(previousStepFile string) (string, error) {
	var yttFiles []string

	yttFiles = append(yttFiles, a.yttDataFiles...)

	if previousStepFile != "" {
		yttFiles = append(yttFiles, previousStepFile)
	}

	yttFiles = append(yttFiles, a.e.collectBySubpath(filepath.Join("_env", a.e.g.YttPkgStepDirName))...)

	if len(yttFiles) == 0 {
		log.Debug().Str("app", a.Name).Msg("No ytt files found")
		return "", nil
	}

	log.Debug().Str("step", "global-ytt").Strs("files", yttFiles).Str("app", a.Name).Msg("Collected ytt files")

	yttOutput, err := a.e.g.ytt(yttFiles)
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to render ytt files")
		return "", err
	}

	if yttOutput.Stdout == "" {
		log.Warn().Str("app", a.Name).Msg("Empty ytt output")
		return "", nil
	}

	return yttOutput.Stdout, nil
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
func (a *Application) storeStepResult(output string, stepName string, stepNumber uint, previousStepFile string) (string, error) {
	fileName := filepath.Join("steps", fmt.Sprintf("%02d-%s.yaml", stepNumber, stepName))
	file := a.expandTempPath(fileName)
	// if previousStepFile is not empty, we need to prepend the output of the previous step
	if previousStepFile != "" {
		previousStepOutput, err := os.ReadFile(previousStepFile)
		if err != nil {
			return "", err
		}
		output = string(previousStepOutput) + output
	}
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
