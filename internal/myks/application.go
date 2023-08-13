package myks

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

const (
	renderStepName    = "render"
	syncStepName      = "sync"
	globalYttStepName = "global-ytt"
	yttStepName       = "ytt"
	yttPkgStepName    = "ytt-pkg"
	helmStepName      = "helm"
	sliceStepName     = "slice"
	initStepName      = "init"
)

type Application struct {
	Name      string
	Prototype string

	e *Environment

	argoCDEnabled bool
	cached        bool
	yttDataFiles  []string
	yttPkgDirs    []string
}

type HelmConfig struct {
	Namespace    string   `yaml:"namespace"`
	KubeVersion  string   `yaml:"kubeVersion"`
	IncludeCRDs  bool     `yaml:"includeCRDs"`
	Capabilities []string `yaml:"capabilities"`
}

var (
	ErrNoVendirConfig    = errors.New("no vendir config found")
	ApplicationLogFormat = "\033[1m[%s > %s > %s]\033[0m %s"
)

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

	dataYaml, err := a.renderDataYaml(append(a.e.g.extraYttPaths, a.yttDataFiles...))
	if err != nil {
		return err
	}

	type ArgoCD struct {
		Enabled bool
	}
	type Cache struct {
		Enabled bool
	}
	type YttPkg struct {
		Dirs []string `yaml:"dirs"`
	}

	var applicationData struct {
		Application struct {
			ArgoCD ArgoCD `yaml:"argocd"`
			Cache  Cache  `yaml:"cache"`
			YttPkg YttPkg `yaml:"yttPkg"`
		}
	}

	err = yaml.Unmarshal(dataYaml, &applicationData)
	if err != nil {
		return err
	}
	a.argoCDEnabled = applicationData.Application.ArgoCD.Enabled
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

func (a *Application) Msg(step string, msg string) string {
	formattedMessage := fmt.Sprintf(ApplicationLogFormat, a.e.Id, a.Name, step, msg)
	return formattedMessage
}

func (a *Application) runCmd(purpose string, cmd string, stdin io.Reader, args []string) (CmdResult, error) {
	return runCmd(cmd, stdin, args, func(cmd string, args []string) {
		log.Debug().Msg(msgRunCmd(purpose, cmd, args))
	})
}

func (a *Application) renderDataYaml(dataFiles []string) ([]byte, error) {
	if len(dataFiles) == 0 {
		return nil, errors.New("No data files found")
	}
	res, err := runYttWithFilesAndStdin(dataFiles, nil, func(name string, args []string) {
		log.Debug().Msg(a.Msg("init", msgRunCmd("render application data values file", name, args)))
	}, "--data-values-inspect")
	if err != nil {
		log.Error().Err(err).Str("stderr", res.Stderr).Msg(a.Msg("init", "Unable to render data"))
		return nil, err
	}
	if res.Stdout == "" {
		return nil, errors.New("Empty output from ytt")
	}

	dataYaml := []byte(res.Stdout)
	return dataYaml, nil
}

func (a *Application) mergeValuesYaml(valueFilesYaml string) (CmdResult, error) {
	return runYttWithFilesAndStdin(nil, nil, func(name string, args []string) {
		log.Debug().Msg(msgRunCmd("merge data values file", name, args))
	}, "--data-values-file="+valueFilesYaml, "--data-values-inspect")
}

func (a *Application) ytt(step string, purpose string, paths []string, args ...string) (CmdResult, error) {
	return a.yttS(step, purpose, paths, nil, args...)
}

func (a *Application) yttS(step string, purpose string, paths []string, stdin io.Reader, args ...string) (CmdResult, error) {
	return runYttWithFilesAndStdin(append(a.e.g.extraYttPaths, paths...), stdin, func(name string, args []string) {
		log.Debug().Msg(a.Msg(step, msgRunCmd(purpose, name, args)))
	}, args...)
}
