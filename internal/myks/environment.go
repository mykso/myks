package myks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

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
		log.Warn().Err(err).Str("dir", e.Dir).Msg("Unable to initialize environment data")
		return err
	}

	e.initApplications(applicationNames)

	return nil
}

func (e *Environment) Sync() error {
	return processItemsInParallel(e.Applications, func(item interface{}) error {
		app, ok := item.(*Application)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Application")
		}
		return app.Sync()
	})
}

func (e *Environment) Render() error {
	return processItemsInParallel(e.Applications, func(item interface{}) error {
		app, ok := item.(*Application)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Application")
		}
		return app.Render()
	})
}

func (e *Environment) SyncAndRender() error {
	return processItemsInParallel(e.Applications, func(item interface{}) error {
		app, ok := item.(*Application)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Application")
		}
		if err := app.Sync(); err != nil {
			return err
		}
		return app.Render()
	})
}

func (e *Environment) setId() error {
	yamlBytes, err := os.ReadFile(e.EnvironmentDataFile)
	if err != nil {
		log.Debug().Err(err).Msg("Unable to read environment data file")
		return err
	}

	var envData struct {
		Environment struct {
			Id string
		}
	}
	err = yaml.Unmarshal(yamlBytes, &envData)
	if err != nil {
		log.Debug().Err(err).Msg("Unable to unmarshal environment data file")
		return err
	}

	log.Debug().Interface("envData", envData).Msg("Environment data")

	if envData.Environment.Id == "" {
		err = errors.New("Environment data file missing id")
		log.Debug().Err(err).Str("file", e.EnvironmentDataFile).Msg("Unable to set environment id")
		return err
	}

	e.Id = envData.Environment.Id

	return nil
}

func (e *Environment) initEnvData() error {
	envDataFiles := e.collectBySubpath(e.g.EnvironmentDataFileName)
	envDataYaml, err := e.renderEnvData(envDataFiles)
	if err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg("Unable to render environment data")
		return err
	}
	err = e.saveRenderedEnvData(envDataYaml)
	if err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg("Unable to save rendered environment data")
		return err
	}
	err = e.setEnvDataFromYaml(envDataYaml)
	if err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg("Unable to set environment data")
		return err
	}

	return nil
}

func (e *Environment) renderEnvData(envDataFiles []string) ([]byte, error) {
	if len(envDataFiles) == 0 {
		return nil, errors.New("No environment data files found")
	}
	res, err := e.g.ytt(envDataFiles, "--data-values-inspect")
	if err != nil {
		log.Error().Err(err).Str("stderr", res.Stderr).Msg("Unable to render environment data")
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
		log.Error().Err(err).Str("dir", dir).Msg("Unable to create directory for rendered envData file")
		return err
	}
	err = os.WriteFile(e.renderedEnvDataFilePath, envDataYaml, 0o600)
	if err != nil {
		log.Error().Err(err).Msg("Unable to write rendered envData file")
		return err
	}
	log.Debug().Str("file", e.renderedEnvDataFilePath).Msg("Wrote rendered envData file")
	return nil
}

func (e *Environment) setEnvDataFromYaml(envDataYaml []byte) error {
	var envDataStruct struct {
		Environment struct {
			Applications []struct {
				Name  string
				Proto string
			}
		}
	}
	err := yaml.Unmarshal(envDataYaml, &envDataStruct)
	if err != nil {
		log.Error().Err(err).Msg("Unable to unmarshal environment data yaml")
		return err
	}

	for _, app := range envDataStruct.Environment.Applications {
		proto := app.Proto
		if len(proto) == 0 {
			log.Error().Interface("app", app).Msg("Application prototype is not set")
			continue
		}

		name := app.Name
		if len(name) == 0 {
			name = proto
		}

		if _, ok := e.foundApplications[name]; ok {
			log.Error().Str("app_name", name).Msg("Duplicated application")
			continue

		}

		e.foundApplications[name] = proto
	}

	if len(e.foundApplications) == 0 {
		log.Warn().Str("dir", e.Dir).Msg("No applications found")
	} else {
		log.Debug().Interface("apps", e.foundApplications).Msg("Found applications")
	}

	return nil
}

func (e *Environment) initApplications(applicationNames []string) {
	for name, proto := range e.foundApplications {
		if len(applicationNames) == 0 || contains(applicationNames, name) {
			app, err := NewApplication(e, name, proto)
			if err != nil {
				log.Warn().Err(err).Str("dir", e.Dir).Interface("app", name).Msg("Unable to initialize application")
			} else {
				e.Applications = append(e.Applications, app)
			}
		}
	}
	log.Debug().Interface("applications", e.Applications).Msg("Applications")
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
