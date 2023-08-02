package myks

import (
	"errors"
	yaml "gopkg.in/yaml.v3"
	"os"
	"path/filepath"
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
	yttPkgDirs   []string
}

type HelmConfig struct {
	Namespace   string `yaml:"namespace"`
	KubeVersion string `yaml:"kubeVersion"`
	IncludeCRDs bool   `yaml:"includeCRDs"`
}

var ErrNoVendirConfig = errors.New("no vendir config found")

func NewApplication(e *Environment, name string, prototypeName string) (*Application, error) {
	if prototypeName == "" {
		prototypeName = name
	}

	prototype := filepath.Join(e.g.PrototypesDir, prototypeName)

	if _, err := os.Stat(prototype); err != nil {
		return nil, errors.New("application prototype does not exist")
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

	dataYaml, err := renderDataYaml(a.Name, append(a.e.g.extraYttPaths, a.yttDataFiles...))
	if err != nil {
		return err
	}

	type Cache struct {
		Enabled bool
	}
	type YttPkg struct {
		Dirs []string `yaml:"dirs"`
	}

	var applicationData struct {
		Application struct {
			Cache  Cache  `yaml:"cache"`
			YttPkg YttPkg `yaml:"yttPkg"`
		}
	}

	err = yaml.Unmarshal(dataYaml, &applicationData)
	if err != nil {
		return err
	}
	a.cached = applicationData.Application.Cache.Enabled
	a.yttPkgDirs = applicationData.Application.YttPkg.Dirs

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

func (a *Application) writeServiceFile(name string, content string) error {
	return writeFile(a.expandServicePath(name), []byte(content))
}

func (a *Application) writeTempFile(name string, content string) error {
	return writeFile(a.expandTempPath(name), []byte(content))
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
