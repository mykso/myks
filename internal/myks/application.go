package myks

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

const (
	renderStepName          = "render"
	syncStepName            = "sync"
	globalYttStepName       = "global-ytt"
	yttStepName             = "ytt"
	yttPkgStepName          = "ytt-pkg"
	helmStepName            = "helm"
	sliceStepName           = "slice"
	initStepName            = "init"
	applyStepName           = "apply"
	crdPrefix               = "customresourcedefinition-"
	namespaceResourcePrefix = "namespace-"
)

type Application struct {
	Name      string
	Prototype string

	e *Environment

	includeNamespace bool
	yttPkgDirs       []string
	yttDataFiles     []string
	ArgoConfig       *ArgoConfig
	HelmConfig       *HelmConfig
}

type HelmConfig struct {
	Namespace         string   `yaml:"namespace"`
	KubeVersion       string   `yaml:"kubeVersion"`
	IncludeCRDs       bool     `yaml:"includeCRDs"`
	Capabilities      []string `yaml:"capabilities"`
	BuildDependencies bool     `yaml:"buildDependencies"`
}

type Destination struct {
	Namespace string `yaml:"namespace"`
}

type App struct {
	Destination Destination `yaml:"destination"`
}

type ArgoConfig struct {
	Enabled bool `yaml:"enabled"`
	App     App  `yaml:"app"`
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

	app := &Application{
		Name:      name,
		Prototype: prototype,
		e:         e,
	}

	if ok, err := isExist(app.Prototype); err != nil {
		return app, err
	} else if !ok {
		return app, errors.New("application prototype does not exist")
	}

	err := app.Init()
	return app, err
}

func (a *Application) Init() error {
	// 1. Collect all ytt data files:
	//    - environment data files: `envs/**/env-data.ytt.yaml`
	//    - application prototype data file: `prototypes/<prototype>/app-data.ytt.yaml`
	//    - application data files: `envs/**/_apps/<app>/add-data.ytt.yaml`

	a.collectDataFiles()

	dataYaml, err := a.renderDataYaml(concatenate(a.e.g.extraYttPaths, a.yttDataFiles))
	if err != nil {
		return err
	}

	type ArgoCD struct {
		Enabled bool `yaml:"enabled"`
	}

	var applicationData struct {
		YttPkg struct {
			Dirs []string `yaml:"dirs"`
		} `yaml:"yttPkg"`
		ArgoCD ArgoCD `yaml:"argocd"`
		Render struct {
			IncludeNamespace bool `yaml:"includeNamespace"`
		} `yaml:"render"`
	}

	err = yaml.Unmarshal(dataYaml, &applicationData)
	if err != nil {
		return err
	}
	a.includeNamespace = applicationData.Render.IncludeNamespace
	a.yttPkgDirs = applicationData.YttPkg.Dirs

	// Transfer ArgoConfig if enabled
	var argoData struct {
		ArgoConfig ArgoConfig `yaml:"argocd"`
	}
	err = yaml.Unmarshal(dataYaml, &argoData)
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(helmStepName, "Unable to unmarshal argo config"))
		return err
	}
	if argoData.ArgoConfig.Enabled {
		a.ArgoConfig = &argoData.ArgoConfig
	}

	// Transfer HelmConfig
	var helmConfig struct {
		Helm HelmConfig
	}
	err = yaml.Unmarshal(dataYaml, &helmConfig)
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(helmStepName, "Unable to unmarshal helm config"))
		return err
	}
	a.HelmConfig = &helmConfig.Helm

	return nil
}

func (a *Application) expandPath(path string) string {
	return filepath.Join(a.e.Dir, a.e.g.AppsDir, a.Name, path)
}

func (a *Application) expandServicePath(path string) string {
	return filepath.Join(a.e.Dir, a.e.g.AppsDir, a.Name, a.e.g.ServiceDirName, path)
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
	if ok, err := isExist(protoDataFile); ok && err == nil {
		a.yttDataFiles = append(a.yttDataFiles, protoDataFile)
	}

	protoOverrideDataFiles := a.e.collectBySubpath(filepath.Join(a.e.g.PrototypeOverrideDir, a.prototypeDirName(), a.e.g.ApplicationDataFileName))
	a.yttDataFiles = append(a.yttDataFiles, protoOverrideDataFiles...)
	overrideDataFiles := append(protoOverrideDataFiles, a.e.collectBySubpath(filepath.Join(a.e.g.AppsDir, a.Name, a.e.g.ApplicationDataFileName))...)
	a.yttDataFiles = append(a.yttDataFiles, overrideDataFiles...)
}

func (a *Application) Msg(step string, msg string) string {
	formattedMessage := fmt.Sprintf(ApplicationLogFormat, a.e.Id, a.Name, step, msg)
	return formattedMessage
}

func (a *Application) runCmd(step, purpose, cmd string, stdin io.Reader, args []string) (CmdResult, error) {
	return runCmd(cmd, stdin, args, func(cmd string, err error, stderr string, args []string) {
		cmd = msgRunCmd(purpose, cmd, args)
		a.logCmd(step, cmd, err, stderr)
	})
}

func (a *Application) logCmd(step string, cmd string, err error, stderr string) {
	if err != nil {
		log.Error().Msg(a.Msg(step, cmd))
		log.Error().Msg(a.Msg(step, stderr))
	} else {
		log.Debug().Msg(a.Msg(step, cmd))
	}
}

func (a *Application) renderDataYaml(dataFiles []string) ([]byte, error) {
	args := []string{
		"-v", "myks.context.step=init", // in the logs this step is called init
		"-v", "myks.context.app=" + a.Name,
		"-v", "myks.context.prototype=" + a.Prototype,
		"--data-values-inspect",
	}
	if len(dataFiles) == 0 {
		return nil, errors.New("No data files found")
	}
	step := "init"
	res, err := runYttWithFilesAndStdin(dataFiles, nil, func(name string, err error, stderr string, args []string) {
		cmd := msgRunCmd("render application data values file", name, args)
		a.logCmd(step, cmd, err, stderr)
	}, args...)
	if err != nil {
		return nil, err
	}
	if res.Stdout == "" {
		return nil, errors.New("Empty output from ytt")
	}

	dataYaml := []byte(res.Stdout)
	return dataYaml, nil
}

func (a *Application) mergeValuesYaml(step string, valueFilesYaml string) (CmdResult, error) {
	return runYttWithFilesAndStdin(nil, nil, func(name string, err error, stderr string, args []string) {
		cmd := msgRunCmd("merge data values file", name, args)
		a.logCmd(step, cmd, err, stderr)
	}, "--data-values-file="+valueFilesYaml, "--data-values-inspect")
}

func (a *Application) ytt(step, purpose string, paths []string, args ...string) (CmdResult, error) {
	return a.yttS(step, purpose, paths, nil, args...)
}

func (a *Application) yttS(step string, purpose string, paths []string, stdin io.Reader, args ...string) (CmdResult, error) {
	args = append(args,
		"-v", "myks.context.step="+step,
		"-v", "myks.context.app="+a.Name,
		"-v", "myks.context.prototype="+a.Prototype)
	paths = concatenate(a.e.g.extraYttPaths, paths)
	return runYttWithFilesAndStdin(paths, stdin, func(name string, err error, stderr string, args []string) {
		cmd := msgRunCmd(purpose, name, args)
		a.logCmd(step, cmd, err, stderr)
	}, args...)
}

func (a *Application) prototypeDirName() string {
	return strings.TrimPrefix(a.Prototype, a.e.g.PrototypesDir+string(filepath.Separator))
}
