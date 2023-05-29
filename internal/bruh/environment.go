package bruh

import (
	"errors"
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
	// Environment manifest
	Manifest *EnvironmentManifest

	// Globe instance
	g *Globe

	// Runtime data
	applications             []*Application
	renderedManifestFilePath string
}

func NewEnvironment(g *Globe, dir string) *Environment {
	envDataFile := filepath.Join(dir, g.EnvironmentDataFileName)

	env := &Environment{
		Dir:                 dir,
		EnvironmentDataFile: envDataFile,
		Manifest: &EnvironmentManifest{
			Applications: []ManifestApplication{},
		},
		g:                        g,
		renderedManifestFilePath: filepath.Join(dir, g.ServiceDirName, g.EnvironmentManifestFileName),
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
	if err := e.initManifest(); err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg("Unable to initialize environment manifest")
		return err
	}

	e.initApplications(applicationNames)

	return nil
}

func (e *Environment) Sync() error {
	for _, app := range e.applications {
		if err := app.Sync(); err != nil {
			return err
		}
	}
	return nil
}

func (e *Environment) Render() error {
	for _, app := range e.applications {
		if err := app.Render(); err != nil {
			return err
		}
	}
	return nil
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

func (e *Environment) initManifest() error {
	manifestFiles := e.getManifestFiles()
	manifestYaml, err := e.renderManifest(manifestFiles)
	if err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg("Unable to render environment manifest")
		return err
	}
	err = e.saveRenderedManifest(manifestYaml)
	if err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg("Unable to save rendered environment manifest")
		return err
	}
	err = e.setManifestFromYaml(manifestYaml)
	if err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg("Unable to set environment manifest")
		return err
	}
	return nil
}

// Get all environment manifest template files up to the root directory
func (e *Environment) getManifestFiles() []string {
	manifestFiles := e.collectBySubpath(e.g.EnvironmentManifestTemplateFileName)
	log.Debug().Interface("manifestFiles", manifestFiles).Msg("Manifest files")
	return manifestFiles
}

// Render the final manifest file for the environment
func (e *Environment) renderManifest(manifestFiles []string) (manifestYaml []byte, err error) {
	if len(manifestFiles) == 0 {
		return nil, errors.New("No manifest files found")
	}
	res, err := e.g.ytt(manifestFiles)
	if err != nil {
		log.Error().Err(err).Str("stderr", res.Stderr).Msg("Unable to render environment manifest")
		return nil, err
	}
	if res.Stdout == "" {
		return nil, errors.New("Empty output from ytt")
	}

	manifestYaml = []byte(res.Stdout)
	return manifestYaml, nil
}

func (e *Environment) saveRenderedManifest(manifestYaml []byte) error {
	dir := filepath.Dir(e.renderedManifestFilePath)
	err := os.MkdirAll(dir, 0o750)
	if err != nil {
		log.Error().Err(err).Str("dir", dir).Msg("Unable to create directory for rendered manifest file")
		return err
	}
	err = os.WriteFile(e.renderedManifestFilePath, manifestYaml, 0o600)
	if err != nil {
		log.Error().Err(err).Msg("Unable to write rendered manifest file")
		return err
	}
	log.Debug().Str("file", e.renderedManifestFilePath).Msg("Wrote rendered manifest file")
	return nil
}

func (e *Environment) setManifestFromYaml(manifestYaml []byte) error {
	var manifestData struct {
		Applications map[string]struct {
			Prototype string
		}
	}
	err := yaml.Unmarshal(manifestYaml, &manifestData)
	if err != nil {
		log.Error().Err(err).Msg("Unable to unmarshal manifest yaml")
		return err
	}

	for appName, appData := range manifestData.Applications {
		e.Manifest.Applications = append(e.Manifest.Applications, ManifestApplication{
			Name:      appName,
			Prototype: appData.Prototype,
		})
	}

	log.Debug().Interface("manifest", e.Manifest).Msg("Manifest")
	return nil
}

func (e *Environment) initApplications(applicationNames []string) {
	for _, manifestApp := range e.Manifest.Applications {
		if len(applicationNames) == 0 || contains(applicationNames, manifestApp.Name) {
			app, err := NewApplication(e, manifestApp.Name, manifestApp.Prototype)
			if err != nil {
				log.Warn().Err(err).Str("dir", e.Dir).Interface("app", manifestApp).Msg("Unable to initialize application")
			} else {
				e.applications = append(e.applications, app)
			}
		}
	}
	log.Debug().Interface("applications", e.applications).Msg("Applications")
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
