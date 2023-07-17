package myks

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

type Application struct {
	// Name of the application
	Name string
	// Application prototype directory
	Prototype string
	// Environment
	e *Environment
	// YTT data files
	yttDataFiles []string
	cached       bool
}

type HelmConfig struct {
	Namespace   string `yaml:"namespace"`
	KubeVersion string `yaml:"kubeVersion"`
	IncludeCRDs bool   `yaml:"includeCRDs"`
}

var ErrNoVendirConfig = errors.New("No vendir config found")

func NewApplication(e *Environment, name string, prototypeName string) (*Application, error) {
	if prototypeName == "" {
		prototypeName = name
	}

	prototype := filepath.Join(e.g.PrototypesDir, prototypeName)

	if _, err := os.Stat(prototype); err != nil {
		return nil, errors.New("Application prototype does not exist")
	}

	app := &Application{
		Name:      name,
		Prototype: prototype,
		e:         e,
	}
	err := app.Init()
	if err != nil {
		return nil, err
	}

	return app, nil
}

func (a *Application) Init() error {
	// 1. Collect all ytt data files:
	//    - environment data files: `envs/**/env-data.ytt.yaml`
	//    - application prototype data file: `prototypes/<prototype>/app-data.ytt.yaml`
	//    - application data files: `envs/**/_apps/<app>/add-data.ytt.yaml`

	a.collectDataFiles()

	dataYaml, err := renderDataYaml(append(a.e.g.extraYttPaths, a.yttDataFiles...))
	if err != nil {
		return err
	}

	var applicationData struct {
		Application struct {
			Cache struct {
				Enabled bool
			}
		}
	}

	err = yaml.Unmarshal(dataYaml, &applicationData)
	if err != nil {
		return err
	}
	a.cached = applicationData.Application.Cache.Enabled

	return nil
}

func (a *Application) Sync() error {
	if err := a.prepareSync(); err != nil {
		if err == ErrNoVendirConfig {
			return nil
		}
		return err
	}

	if err := a.doSync(); err != nil {
		return err
	}

	return nil
}

func (a *Application) Render() error {

	log.Debug().Strs("files", a.yttDataFiles).Msg("Collected ytt data files")

	// 2. Run built-in rendering steps:
	//   a. helm
	//   b. ytt
	//   c. global ytt

	outputYaml, err := a.runHelm()
	if err != nil {
		return err
	}

	helmStepOutputFile := ""
	if outputYaml != "" {
		helmStepOutputFile, err = a.storeStepResult(outputYaml, "helm", 1)
		if err != nil {
			log.Error().Err(err).Msg("Failed to store helm step result")
			return err
		}
	}

	outputYaml, err = a.runYtt(helmStepOutputFile)
	if err != nil {
		return err
	}

	yttStepOutputFile, err := a.storeStepResult(outputYaml, "ytt", 2)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store ytt step result")
		return err
	}

	outputYaml, err = a.runGlobalYtt(yttStepOutputFile)
	if err != nil {
		return err
	}

	globalYttStepOutputFile, err := a.storeStepResult(outputYaml, "global.ytt", 3)
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

func (a *Application) prepareSync() error {
	// Collect ytt arguments following the following steps:
	// 1. If exists, use the `apps/<prototype>/vendir` directory.
	// 2. If exists, for every level of environments use `<env>/_apps/<app>/vendir` directory.

	yttFiles := []string{}

	protoVendirDir := filepath.Join(a.Prototype, "vendir")
	if _, err := os.Stat(protoVendirDir); err == nil {
		yttFiles = append(yttFiles, protoVendirDir)
		log.Debug().Str("dir", protoVendirDir).Msg("Using prototype vendir directory")
	}

	appVendirDirs := a.e.collectBySubpath(filepath.Join("_apps", a.Name, "vendir"))
	yttFiles = append(yttFiles, appVendirDirs...)

	if len(yttFiles) == 0 {
		err := ErrNoVendirConfig
		log.Warn().Err(err).Str("app", a.Name).Msg("")
		return err
	}

	vendirConfig, err := a.e.g.ytt(yttFiles)
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to render vendir config")
		return err
	}

	if vendirConfig.Stdout == "" {
		err = errors.New("Empty vendir config")
		log.Warn().Err(err).Msg("")
		return err
	}

	vendirConfigFilePath := a.expandServicePath(a.e.g.VendirConfigFileName)
	// Create directory if it does not exist
	err = os.MkdirAll(filepath.Dir(vendirConfigFilePath), 0o750)
	if err != nil {
		log.Warn().Err(err).Msg("Unable to create directory for vendir config file")
		return err
	}
	err = os.WriteFile(vendirConfigFilePath, []byte(vendirConfig.Stdout), 0o600)
	if err != nil {
		log.Warn().Err(err).Msg("Unable to write vendir config file")
		return err
	}
	log.Debug().Str("file", vendirConfigFilePath).Msg("Wrote vendir config file")

	return nil
}

func (a *Application) expandPath(path string) string {
	return filepath.Join(a.e.Dir, "_apps", a.Name, path)
}

func (a *Application) expandServicePath(path string) string {
	return filepath.Join(a.e.Dir, "_apps", a.Name, a.e.g.ServiceDirName, path)
}

func (a *Application) expandTempPath(path string) string {
	return a.expandServicePath(filepath.Join(a.e.g.TempDirName, path))
}

// TODO: for content, use []byte instead of string
func (a *Application) writeFile(path string, content string) error {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); err != nil {
		err := os.MkdirAll(dir, 0o750)
		if err != nil {
			log.Warn().Err(err).Msg("Unable to create directory")
			return err
		}
	}

	return os.WriteFile(path, []byte(content), 0o600)
}

func (a *Application) writeServiceFile(name string, content string) error {
	return a.writeFile(a.expandServicePath(name), content)
}

func (a *Application) writeTempFile(name string, content string) error {
	return a.writeFile(a.expandTempPath(name), content)
}

func (a *Application) collectDataFiles() {
	environmentDataFiles := a.e.collectBySubpath(a.e.g.EnvironmentDataFileName)
	a.yttDataFiles = append(a.yttDataFiles, environmentDataFiles...)

	protoDataFile := filepath.Join(a.Prototype, a.e.g.ApplicationDataFileName)
	if _, err := os.Stat(protoDataFile); err == nil {
		a.yttDataFiles = append(a.yttDataFiles, protoDataFile)
	}

	overrideDataFiles := a.e.collectBySubpath(filepath.Join("_apps", a.Name, a.e.g.ApplicationDataFileName))
	a.yttDataFiles = append(a.yttDataFiles, overrideDataFiles...)
}

func (a *Application) runHelm() (string, error) {
	chartDirs := a.getHelmChartDirs()
	if len(chartDirs) == 0 {
		log.Debug().Str("app", a.Name).Msg("No charts to process")
		return "", nil
	}

	helmConfig, err := a.getHelmConfig()
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to get helm config")
		return "", err
	}

	commonHelmArgs := []string{}

	// FIXME: move Namespace to a per-chart config
	if helmConfig.Namespace == "" {
		helmConfig.Namespace = a.e.g.NamespacePrefix + a.Name
	}
	commonHelmArgs = append(commonHelmArgs, "--namespace", helmConfig.Namespace)

	if helmConfig.KubeVersion != "" {
		commonHelmArgs = append(commonHelmArgs, "--kube-version", helmConfig.KubeVersion)
	}

	// FIXME: move IncludeCRDs to a per-chart config
	if helmConfig.IncludeCRDs {
		commonHelmArgs = append(commonHelmArgs, "--include-crds")
	}

	outputs := []string{}

	for _, chartDir := range chartDirs {
		chartName := filepath.Base(chartDir)
		if err := a.prepareHelm(chartName); err != nil {
			log.Warn().Err(err).Str("app", a.Name).Msg("Unable to prepare helm values")
			return "", err
		}

		// FIXME: replace a.Name with a name of the chart being processed
		helmArgs := []string{
			"template",
			"--skip-tests",
			chartName,
			chartDir,
		}

		helmValuesFile := a.expandServicePath(a.getHelmValuesFileName(chartName))
		if _, err := os.Stat(helmValuesFile); err == nil {
			helmArgs = append(helmArgs, "--values", helmValuesFile)
		} else {
			log.Debug().Str("app", a.Name).Str("chart", chartName).Msg("No helm values file")
		}

		res, err := runCmd("helm", nil, append(helmArgs, commonHelmArgs...))
		if err != nil {
			log.Warn().Err(err).Str("stdout", res.Stdout).Str("stderr", res.Stderr).Msg("Unable to run helm")
			return "", err
		}

		if res.Stdout == "" {
			log.Warn().Str("app", a.Name).Str("chart", chartName).Msg("No helm output")
			continue
		}

		outputs = append(outputs, res.Stdout)

	}

	return strings.Join(outputs, "---\n"), nil
}

func (a *Application) getHelmChartsDir() (string, error) {
	chartsDir := a.expandPath(filepath.Join(a.e.g.VendorDirName, a.e.g.HelmChartsDirName))
	if _, err := os.Stat(chartsDir); err != nil {
		if os.IsNotExist(err) {
			log.Debug().Str("dir", chartsDir).Msg("Helm charts directory does not exist")
			return "", nil
		}
		log.Warn().Err(err).Str("dir", chartsDir).Msg("Unable to stat helm charts directory")
		return "", err
	}

	return chartsDir, nil
}

func (a *Application) getHelmChartDirs() []string {
	chartsDir, err := a.getHelmChartsDir()
	if err != nil || chartsDir == "" {
		return []string{}
	}

	chartDirs := []string{}
	err = filepath.Walk(chartsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("Unable to walk helm charts directory")
			return err
		}

		if info.IsDir() && path != chartsDir {
			chartDirs = append(chartDirs, path)
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		log.Warn().Err(err).Msg("Unable to walk helm charts directory")
		return []string{}
	}

	return chartDirs
}

// prepareHelm generates values.yaml file from ytt data files and ytt templates
// from the `helm` directories of the prototype and the application.
func (a *Application) prepareHelm(chartName string) error {
	helmValuesFileName := a.getHelmValuesFileName(chartName)

	helmYttFiles := []string{}

	prototypeHelmValues := filepath.Join(a.Prototype, helmValuesFileName)
	if _, err := os.Stat(prototypeHelmValues); err == nil {
		helmYttFiles = append(helmYttFiles, prototypeHelmValues)
	}

	helmYttFiles = append(helmYttFiles, a.e.collectBySubpath(filepath.Join("_apps", a.Name, helmValuesFileName))...)

	if len(helmYttFiles) == 0 {
		log.Debug().Str("app", a.Name).Str("chart", chartName).Msg("No helm values templates found, helm values file will not be generated")
		return nil
	}

	log.Debug().Strs("files", helmYttFiles).Str("app", a.Name).Str("chart", chartName).Msg("Collected helm values templates")

	helmValuesYamls, err := a.e.g.ytt(append(a.yttDataFiles, helmYttFiles...))
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to render helm values templates")
		return err
	}

	if helmValuesYamls.Stdout == "" {
		log.Warn().Str("app", a.Name).Msg("Empty helm values")
		return nil
	}

	err = a.writeTempFile(helmValuesFileName, helmValuesYamls.Stdout)
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to write helm values file")
		return err
	}

	helmValues, err := mergeValuesYaml(a.expandTempPath(helmValuesFileName))
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to render helm values")
		return err
	}

	if helmValues.Stdout == "" {
		log.Warn().Str("app", a.Name).Msg("Empty helm values")
		return nil
	}

	return a.writeServiceFile(helmValuesFileName, helmValues.Stdout)
}

func (a *Application) getHelmConfig() (HelmConfig, error) {
	dataValuesYaml, err := a.e.g.ytt(a.yttDataFiles, "--data-values-inspect")
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to inspect data values")
		return HelmConfig{}, err
	}

	var helmConfig struct {
		Helm HelmConfig
	}
	err = yaml.Unmarshal([]byte(dataValuesYaml.Stdout), &helmConfig)
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to unmarshal data values")
		return HelmConfig{}, err
	}

	return helmConfig.Helm, nil
}

func (a *Application) getHelmValuesFileName(chartName string) string {
	return filepath.Join("helm", chartName+".yaml")
}

func (a *Application) runYtt(previousStepFile string) (string, error) {
	yttFiles := []string{}

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
		log.Debug().Str("app", a.Name).Msg("No ytt files found")
		return "", nil
	}

	log.Debug().Strs("files", yttFiles).Str("app", a.Name).Msg("Collected ytt files")

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

func (a *Application) runGlobalYtt(previousStepFile string) (string, error) {
	yttFiles := []string{}

	yttFiles = append(yttFiles, a.yttDataFiles...)

	if previousStepFile != "" {
		yttFiles = append(yttFiles, previousStepFile)
	}

	yttFiles = append(yttFiles, a.e.collectBySubpath(filepath.Join("_env", a.e.g.YttStepDirName))...)

	if len(yttFiles) == 0 {
		log.Debug().Str("app", a.Name).Msg("No ytt files found")
		return "", nil
	}

	log.Debug().Strs("files", yttFiles).Str("app", a.Name).Msg("Collected ytt files")

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
		err = a.writeFile(filePath, data.String())
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
	if n, ok := resource["metadata"].(map[string]interface{})["name"]; ok {
		name = n.(string)
	}
	return fmt.Sprintf("%s-%s.yaml", strings.ToLower(kind), strings.ToLower(name))
}
