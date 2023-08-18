package myks

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

var EnvLogFormat = "\033[1m[%s > %s]\033[0m %s"

type ManifestApplication struct {
	Name      string
	Prototype string
}

type EnvironmentManifest struct {
	Applications []ManifestApplication
}

type Environment struct {
	// Path to the environment directory
	Dir string
	// Environment data file
	EnvironmentDataFile string
	// Environment id
	Id string
	// Applications
	Applications []*Application

	// Globe instance
	g *Globe

	argoCDEnabled bool
	// Runtime data
	renderedEnvDataFilePath string
	// Found applications
	foundApplications map[string]string
}

func NewEnvironment(g *Globe, dir string) *Environment {
	envDataFile := filepath.Join(dir, g.EnvironmentDataFileName)

	env := &Environment{
		Dir:                     dir,
		EnvironmentDataFile:     envDataFile,
		Applications:            []*Application{},
		g:                       g,
		renderedEnvDataFilePath: filepath.Join(dir, g.ServiceDirName, g.RenderedEnvironmentDataFileName),
		foundApplications:       map[string]string{},
	}

	// Read an environment id from an environment data file.
	// The environment data file must exist and contain an .environment.id field.
	if err := env.setId(); err != nil {
		log.Warn().Err(err).Str("dir", dir).Msg("Unable to set environment id")
		return nil
	}

	return env
}

func (e *Environment) Init(applicationNames []string) error {
	if err := e.initEnvData(); err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg(e.Msg("Unable to initialize environment data"))
		return err
	}

	err := e.initApplications(applicationNames)
	if err != nil {
		log.Error().Err(err).Msg(e.Msg("Unable to initialize applications"))
		return err
	}

	return nil
}

func (e *Environment) Sync(asyncLevel int) error {
	return process(asyncLevel, e.Applications, func(item interface{}) error {
		app, ok := item.(*Application)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Application")
		}
		return app.Sync()
	})
}

func (e *Environment) Render(asyncLevel int) error {
	if err := e.renderArgoCD(); err != nil {
		return err
	}
	return process(asyncLevel, e.Applications, func(item interface{}) error {
		app, ok := item.(*Application)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Application")
		}
		yamlTemplatingTools := []YamlTemplatingTool{
			&Helm{ident: "helm", app: app, additive: true},
			&YttPkg{ident: "ytt-pkg", app: app, additive: true},
			&Ytt{ident: "ytt", app: app, additive: false},
			&GlobalYtt{ident: "global-ytt", app: app, additive: false},
		}
		if err := app.RenderAndSlice(yamlTemplatingTools); err != nil {
			return err
		}

		return app.renderArgoCD()
	})
}

func (e *Environment) SyncAndRender(asyncLevel int) error {
	if err := e.renderArgoCD(); err != nil {
		return err
	}
	return process(asyncLevel, e.Applications, func(item interface{}) error {
		app, ok := item.(*Application)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Application")
		}
		if err := app.Sync(); err != nil {
			return err
		}
		yamlTemplatingTools := []YamlTemplatingTool{
			&Helm{ident: "helm", app: app, additive: true},
			&YttPkg{ident: "ytt-pkg", app: app, additive: true},
			&Ytt{ident: "ytt", app: app, additive: false},
			&GlobalYtt{ident: "global-ytt", app: app, additive: false},
		}
		if err := app.RenderAndSlice(yamlTemplatingTools); err != nil {
			return err
		}
		return app.renderArgoCD()
	})
}

func (e *Environment) setId() error {
	yamlBytes, err := os.ReadFile(e.EnvironmentDataFile)
	if err != nil {
		log.Debug().Err(err).Msg(e.Msg("Unable to read environment data file"))
		return err
	}

	var envData struct {
		Environment struct {
			Id string
		}
	}
	err = yaml.Unmarshal(yamlBytes, &envData)
	if err != nil {
		log.Debug().Err(err).Msg(e.Msg("Unable to unmarshal environment data file"))
		return err
	}

	if envData.Environment.Id == "" {
		err = errors.New("Environment data file missing id")
		log.Debug().Err(err).Str("file", e.EnvironmentDataFile).Msg("Unable to set environment id")
		return err
	}

	e.Id = envData.Environment.Id

	log.Debug().Interface("envData", envData).Msg(e.Msg("Environment data"))

	return nil
}

func (e *Environment) initEnvData() error {
	envDataFiles := e.collectBySubpath(e.g.EnvironmentDataFileName)
	envDataYaml, err := e.renderEnvData(envDataFiles)
	if err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg(e.Msg("Unable to render environment data"))
		return err
	}
	err = e.saveRenderedEnvData(envDataYaml)
	if err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg(e.Msg("Unable to save rendered environment data"))
		return err
	}
	err = e.setEnvDataFromYaml(envDataYaml)
	if err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg(e.Msg("Unable to set environment data"))
		return err
	}

	return nil
}

func (e *Environment) renderEnvData(envDataFiles []string) ([]byte, error) {
	if len(envDataFiles) == 0 {
		return nil, errors.New("No environment data files found")
	}
	res, err := e.ytt("render environment data values file", envDataFiles, "--data-values-inspect")
	if err != nil {
		log.Error().Err(err).Str("stderr", res.Stderr).Msg(e.Msg("Unable to render environment data"))
		return nil, err
	}
	if res.Stdout == "" {
		return nil, errors.New("Empty output from ytt")
	}

	envDataYaml := []byte(res.Stdout)
	return envDataYaml, nil
}

func (e *Environment) saveRenderedEnvData(envDataYaml []byte) error {
	dir := filepath.Dir(e.renderedEnvDataFilePath)
	err := os.MkdirAll(dir, 0o750)
	if err != nil {
		log.Error().Err(err).Str("dir", dir).Msg(e.Msg("Unable to create directory for rendered envData file"))
		return err
	}
	err = os.WriteFile(e.renderedEnvDataFilePath, envDataYaml, 0o600)
	if err != nil {
		log.Error().Err(err).Msg(e.Msg("Unable to write rendered envData file"))
		return err
	}
	return nil
}

func (e *Environment) setEnvDataFromYaml(envDataYaml []byte) error {
	var envDataStruct struct {
		ArgoCD struct {
			Enabled bool
		}
		Environment struct {
			Applications []struct {
				Name  string
				Proto string
			}
		}
	}
	err := yaml.Unmarshal(envDataYaml, &envDataStruct)
	if err != nil {
		log.Error().Err(err).Msg(e.Msg("Unable to unmarshal environment data yaml"))
		return err
	}

	e.argoCDEnabled = envDataStruct.ArgoCD.Enabled

	for _, app := range envDataStruct.Environment.Applications {
		proto := app.Proto
		if len(proto) == 0 {
			log.Error().Interface("app", app).Msg(e.Msg("Application prototype is not set"))
			continue
		}

		name := app.Name
		if len(name) == 0 {
			name = proto
		}

		if _, ok := e.foundApplications[name]; ok {
			log.Error().Str("app_name", name).Msg(e.Msg("Duplicated application"))
			continue

		}

		e.foundApplications[name] = proto
	}

	if len(e.foundApplications) == 0 {
		log.Warn().Str("dir", e.Dir).Msg("No applications found")
	} else {
		log.Debug().Interface("apps", e.foundApplications).Msg(e.Msg("Found applications"))
	}

	return nil
}

func (e *Environment) initApplications(applicationNames []string) error {
	if len(applicationNames) == 0 {
		for name, proto := range e.foundApplications {
			app, err := NewApplication(e, name, proto)
			if err != nil {
				log.Warn().Err(err).Str("dir", e.Dir).Interface("app", name).Msg(e.Msg("Unable to initialize application"))
			} else {
				e.Applications = append(e.Applications, app)
			}
		}
	}
	for _, appName := range applicationNames {
		proto := e.foundApplications[appName]
		if proto == "" {
			return errors.New("Application not found: " + appName)
		}
		app, err := NewApplication(e, appName, proto)
		if err != nil {
			log.Warn().Err(err).Str("dir", e.Dir).Interface("app", appName).Msg(e.Msg("Unable to initialize application"))
		} else {
			e.Applications = append(e.Applications, app)
		}
	}
	return nil
}

func (e *Environment) collectBySubpath(subpath string) []string {
	items := []string{}
	currentPath := e.g.RootDir
	levels := []string{""}
	levels = append(levels, strings.Split(e.Dir, filepath.FromSlash("/"))...)
	for _, level := range levels {
		currentPath = filepath.Join(currentPath, level)
		item := filepath.Join(currentPath, subpath)
		if _, err := os.Stat(item); err == nil {
			items = append(items, item)
		}
	}
	return items
}

func (e *Environment) Msg(msg string) string {
	formattedMessage := fmt.Sprintf(EnvLogFormat, e.Id, initStepName, msg)
	return formattedMessage
}

func (e *Environment) ytt(purpose string, paths []string, args ...string) (CmdResult, error) {
	return e.yttS(purpose, paths, nil, args...)
}

func (e *Environment) yttS(purpose string, paths []string, stdin io.Reader, args ...string) (CmdResult, error) {
	return runYttWithFilesAndStdin(append(e.g.extraYttPaths, paths...), stdin, func(name string, args []string) {
		log.Debug().Msg(e.Msg(msgRunCmd(purpose, name, args)))
	}, args...)
}
