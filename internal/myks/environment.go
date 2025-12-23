package myks

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

var EnvLogFormat = "\033[1m[%s > %s]\033[0m %s"

const environmentDataLibTpl = `
#@ def _env_data():
{{ .Data }}
#@ end

#@ load("@ytt:struct", "struct")
#@ load("@ytt:yaml", "yaml")
#@ env_data = struct.encode(yaml.decode(yaml.encode(_env_data())))
`

type Environment struct {
	// Path to the environment directory
	Dir string
	// Environment data file
	EnvironmentDataFile string
	// Environment id
	ID string
	// Applications
	Applications []*Application

	// Globe instance
	g *Globe

	argoCDEnabled bool
	// Runtime data
	renderedDataLibFilePath string
	// Found applications
	foundApplications map[string]string
}

func NewEnvironment(g *Globe, dir string, envDataFile string) (*Environment, error) {
	env := &Environment{
		Dir:                     dir,
		EnvironmentDataFile:     envDataFile,
		Applications:            []*Application{},
		g:                       g,
		renderedDataLibFilePath: filepath.Join(g.RootDir, g.ServiceDirName, dir, g.RenderedEnvironmentDataLibFileName),
		foundApplications:       map[string]string{},
	}

	// Read an environment id from an environment data file.
	// The environment data file must exist and contain an .environment.id field.
	err := env.setID()
	return env, err
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

func (e *Environment) Sync(asyncLevel int, syncTool SyncTool, vendirSecrets string) error {
	return process(asyncLevel, slices.Values(e.Applications), func(app *Application) error {
		return app.Sync(syncTool, vendirSecrets)
	})
}

func (e *Environment) Render(asyncLevel int) error {
	if err := e.renderArgoCD(); err != nil {
		return err
	}
	err := process(asyncLevel, slices.Values(e.Applications), func(app *Application) error {
		yamlTemplatingTools := []YamlTemplatingTool{
			&Helm{ident: "helm", app: app, additive: true},
			&YttPkg{ident: "ytt-pkg", app: app, additive: true},
			&Ytt{ident: "ytt", app: app, additive: false},
			&GlobalYtt{ident: "global-ytt", app: app, additive: false},
			&Kbld{ident: "kbld", app: app, additive: false},
		}
		if err := app.RenderAndSlice(yamlTemplatingTools); err != nil {
			return err
		}

		if err := app.copyStaticFiles(); err != nil {
			return err
		}

		return app.renderArgoCD()
	})
	if err != nil {
		log.Error().Err(err).Msg(e.Msg("Unable to render applications"))
		return err
	}

	return e.Cleanup()
}

func (e *Environment) ExecPlugin(asyncLevel int, p Plugin, args []string) error {
	return process(asyncLevel, slices.Values(e.Applications), func(app *Application) error {
		return p.Exec(app, args)
	})
}

func (e *Environment) Cleanup() error {
	apps, err := e.renderedApplications()
	if err != nil {
		return err
	}
	for _, app := range apps {
		if _, ok := e.foundApplications[app]; !ok {
			log.Info().Str("app", app).Msg(e.Msg("Removing app as it is not configured"))
			err := os.RemoveAll(filepath.Join(e.g.RootDir, e.g.RenderedEnvsDir, e.ID, app))
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("unable to remove dir: %w", err)
			}
			err = os.Remove(filepath.Join(e.g.RootDir, e.g.RenderedArgoDir, e.ID, getArgoCDAppFileName(app)))
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("unable to remove file: %w", err)
			}
		}
	}

	return nil
}

// renderedApplications returns list of applications in rendered dir
func (e *Environment) renderedApplications() ([]string, error) {
	apps := []string{}
	dirEnvRendered := filepath.Join(e.g.RootDir, e.g.RenderedEnvsDir, e.ID)
	files, err := os.ReadDir(dirEnvRendered)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug().Str("dir", dirEnvRendered).Err(err).Msg("")
			return apps, nil
		}
		return nil, fmt.Errorf("unable to read dir: %w", err)
	}
	for _, file := range files {
		if file.IsDir() {
			dir := file.Name()
			apps = append(apps, dir)
		}
	}
	log.Debug().Strs("apps", apps).Msg(e.Msg("Found rendered applications"))
	return apps, nil
}

// find apps that are missing from rendered folder
func (e *Environment) missingApplications() ([]string, error) {
	apps, err := e.renderedApplications()
	if err != nil {
		return nil, err
	}
	missingApps := []string{}
	for app := range e.foundApplications {
		if !slices.Contains(apps, app) {
			missingApps = append(missingApps, app)
		}
	}
	return missingApps, nil
}

func (e *Environment) setID() error {
	yamlBytes, err := os.ReadFile(e.EnvironmentDataFile)
	if err != nil {
		log.Debug().Err(err).Msg(e.Msg("Unable to read environment data file"))
		return err
	}

	var envData struct {
		Environment struct {
			ID string `yaml:"id"`
		} `yaml:"environment"`
	}
	err = yaml.Unmarshal(yamlBytes, &envData)
	if err != nil {
		log.Debug().Err(err).Msg(e.Msg("Unable to unmarshal environment data file"))
		return err
	}

	if envData.Environment.ID == "" {
		err = errors.New("environment data file missing id")
		log.Debug().Err(err).Str("file", e.EnvironmentDataFile).Msg("Unable to set environment id")
		return err
	}

	e.ID = envData.Environment.ID

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
	envDataLib, err := e.renderEnvDataLib(envDataYaml)
	if err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg(e.Msg("Unable to render environment data lib"))
		return err
	}
	err = e.saveRenderedEnvDataLib(envDataLib)
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
		return nil, errors.New("no environment data files found")
	}
	res, err := e.ytt("render environment data values file", envDataFiles, "--data-values-inspect")
	if err != nil {
		return nil, err
	}
	if res.Stdout == "" {
		return nil, errors.New("empty output from ytt")
	}

	envDataYaml := []byte(res.Stdout)
	return envDataYaml, nil
}

func (e *Environment) renderEnvDataLib(envDataYaml []byte) ([]byte, error) {
	tmpl, err := template.New("env_data_lib").Parse(environmentDataLibTpl)
	if err != nil {
		log.Fatal().Err(err).Msg(e.Msg("Unable to parse environment data lib template"))
		return nil, err
	}
	tplData := struct {
		Data string
	}{
		Data: string(envDataYaml),
	}
	renderedLib := &bytes.Buffer{}
	err = tmpl.Execute(renderedLib, tplData)
	if err != nil {
		return nil, err
	}
	return renderedLib.Bytes(), nil
}

func (e *Environment) saveRenderedEnvDataLib(envDataLib []byte) error {
	dir := filepath.Dir(e.renderedDataLibFilePath)
	err := os.MkdirAll(dir, 0o750)
	if err != nil {
		log.Error().Err(err).Str("dir", dir).Msg(e.Msg("Unable to create directory for rendered envData file"))
		return err
	}
	err = os.WriteFile(e.renderedDataLibFilePath, envDataLib, 0o600)
	if err != nil {
		log.Error().Err(err).Msg(e.Msg("Unable to write rendered envData file"))
		return err
	}
	return nil
}

func (e *Environment) setEnvDataFromYaml(envDataYaml []byte) error {
	var envDataStruct struct {
		ArgoCD struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"argocd"`
		Environment struct {
			Applications []struct {
				Name  string `yaml:"name"`
				Proto string `yaml:"proto"`
			} `yaml:"applications"`
		} `yaml:"environment"`
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
				return fmt.Errorf("unable to initialize application %s for env %s. Err: %w", name, e.Dir, err)
			} else {
				e.Applications = append(e.Applications, app)
			}
		}
		return nil
	}
	// applicationNames provided via commandline. Be more friendly
	for _, appName := range applicationNames {
		proto := e.foundApplications[appName]
		if proto == "" {
			log.Warn().Str("dir", e.Dir).Interface("app", appName).Msg(e.Msg("Application not found"))
			continue
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
	envRelDir := strings.TrimPrefix(e.Dir, e.g.RootDir+string(filepath.Separator))
	levels := strings.SplitSeq(envRelDir, filepath.FromSlash("/"))
	for level := range levels {
		if level == "" {
			continue
		}
		currentPath = filepath.Join(currentPath, level)
		item := filepath.Join(currentPath, subpath)
		if files, err := filepath.Glob(item); err == nil {
			items = append(items, files...)
		}
	}
	return items
}

func (e *Environment) Msg(msg string) string {
	formattedMessage := fmt.Sprintf(EnvLogFormat, e.ID, initStepName, msg)
	return formattedMessage
}

func (e *Environment) ytt(purpose string, paths []string, args ...string) (CmdResult, error) {
	return e.yttS(purpose, paths, nil, args...)
}

func (e *Environment) yttS(purpose string, paths []string, stdin io.Reader, args ...string) (CmdResult, error) {
	paths = concatenate(e.g.extraYttPaths, paths)
	return runYttWithFilesAndStdin(paths, stdin, func(name string, err error, stderr string, args []string) {
		cmd := msgRunCmd(purpose, name, args)
		if err != nil {
			log.Error().Msg(e.Msg(cmd))
			log.Error().Msg(e.Msg(stderr))
		} else {
			log.Debug().Msg(e.Msg(cmd))
		}
	}, args...)
}

func (e *Environment) GetApplicationNames() []string {
	var appNames []string
	for _, app := range e.Applications {
		appNames = append(appNames, app.Name)
	}
	return appNames
}

func (e *Environment) getYttLibAPIDir() string {
	return filepath.Join(e.g.RootDir, e.g.ServiceDirName, e.Dir, e.g.YttLibAPIDir)
}
