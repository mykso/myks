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

// EnvLogFormat is the printf format used for environment-level log prefixes.
var EnvLogFormat = "\033[1m[%s > %s]\033[0m %s"

const environmentDataLibTpl = `
#@ def _env_data():
{{ .Data }}
#@ end

#@ load("@ytt:struct", "struct")
#@ load("@ytt:yaml", "yaml")
#@ env_data = struct.encode(yaml.decode(yaml.encode(_env_data())))
`

// Environment represents a Kubernetes cluster configuration context.
type Environment struct {
	// Path to the environment directory
	Dir string
	// Environment data file
	EnvironmentDataFile string
	// Environment id
	ID string
	// Applications
	Applications []*Application

	// Globe instance (kept for runtime state: git data, environments map)
	g *Globe
	// Naming and path configuration (points to g.Config)
	cfg *Config
	// Extra ytt paths copied from Globe at construction time
	extraYttPaths []string

	argoCDEnabled bool
	initialized   bool
	// Runtime data
	renderedDataLibFilePath string
	// Found applications
	foundApplications map[string]string
}

// NewEnvironment creates and partially initializes a new Environment from the given directory and data file.
func NewEnvironment(g *Globe, dir, envDataFile string) (*Environment, error) {
	env := &Environment{
		Dir:                     dir,
		EnvironmentDataFile:     envDataFile,
		Applications:            []*Application{},
		g:                       g,
		cfg:                     &g.Config,
		extraYttPaths:           g.extraYttPaths,
		renderedDataLibFilePath: filepath.Join(g.RootDir, g.ServiceDirName, dir, g.RenderedEnvironmentDataLibFileName),
		foundApplications:       map[string]string{},
	}

	// Read an environment id from an environment data file.
	// The environment data file must exist and contain an .environment.id field.
	err := env.setID()
	return env, err
}

// Init initializes the environment by loading environment data and creating application instances.
func (e *Environment) Init(applicationNames []string) error {
	if err := e.initEnvData(); err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg(e.Msg("Unable to initialize environment data"))
		return fmt.Errorf("initializing environment data for %s: %w", e.Dir, err)
	}

	if err := e.initApplications(applicationNames); err != nil {
		log.Error().Err(err).Msg(e.Msg("Unable to initialize applications"))
		return fmt.Errorf("initializing applications for %s: %w", e.Dir, err)
	}

	e.initialized = true

	return nil
}

// Cleanup removes rendered outputs for apps that are no longer configured.
func (e *Environment) Cleanup() error {
	apps, err := e.renderedApplications()
	if err != nil {
		return err
	}
	for _, app := range apps {
		if _, ok := e.foundApplications[app]; ok {
			continue
		}
		log.Info().Str("app", app).Msg(e.Msg("Removing app as it is not configured"))
		err := os.RemoveAll(filepath.Join(e.cfg.RootDir, e.cfg.RenderedEnvsDir, e.ID, app))
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("unable to remove dir: %w", err)
		}
		err = os.Remove(filepath.Join(e.cfg.RootDir, e.cfg.RenderedArgoDir, e.ID, getArgoCDAppFileName(app)))
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("unable to remove file: %w", err)
		}
	}

	return nil
}

// renderedApplications returns list of applications in rendered dir
func (e *Environment) renderedApplications() ([]string, error) {
	apps := []string{}
	dirEnvRendered := filepath.Join(e.cfg.RootDir, e.cfg.RenderedEnvsDir, e.ID)
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
		return fmt.Errorf("reading environment data file %s: %w", e.EnvironmentDataFile, err)
	}

	var envData struct {
		Environment struct {
			ID string `yaml:"id"`
		} `yaml:"environment"`
	}
	err = yaml.Unmarshal(yamlBytes, &envData)
	if err != nil {
		log.Debug().Err(err).Msg(e.Msg("Unable to unmarshal environment data file"))
		return fmt.Errorf("parsing environment data file %s: %w", e.EnvironmentDataFile, err)
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
	envDataFiles := e.collectBySubpath(e.cfg.EnvironmentDataFileName)
	envDataYaml, err := e.renderEnvData(envDataFiles)
	if err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg(e.Msg("Unable to render environment data"))
		return fmt.Errorf("rendering environment data: %w", err)
	}
	envDataLib, err := e.renderEnvDataLib(envDataYaml)
	if err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg(e.Msg("Unable to render environment data lib"))
		return fmt.Errorf("rendering environment data lib: %w", err)
	}
	if err = e.saveRenderedEnvDataLib(envDataLib); err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg(e.Msg("Unable to save rendered environment data"))
		return fmt.Errorf("saving rendered environment data lib: %w", err)
	}
	if err = e.setEnvDataFromYaml(envDataYaml); err != nil {
		log.Warn().Err(err).Str("dir", e.Dir).Msg(e.Msg("Unable to set environment data"))
		return fmt.Errorf("parsing environment data yaml: %w", err)
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
		return nil, fmt.Errorf("parsing environment data lib template: %w", err)
	}
	tplData := struct {
		Data string
	}{
		Data: string(envDataYaml),
	}
	renderedLib := &bytes.Buffer{}
	if err = tmpl.Execute(renderedLib, tplData); err != nil {
		return nil, fmt.Errorf("executing environment data lib template: %w", err)
	}
	return renderedLib.Bytes(), nil
}

func (e *Environment) saveRenderedEnvDataLib(envDataLib []byte) error {
	dir := filepath.Dir(e.renderedDataLibFilePath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Error().Err(err).Str("dir", dir).Msg(e.Msg("Unable to create directory for rendered envData file"))
		return fmt.Errorf("creating env data lib directory %s: %w", dir, err)
	}
	if err := os.WriteFile(e.renderedDataLibFilePath, envDataLib, 0o600); err != nil {
		log.Error().Err(err).Msg(e.Msg("Unable to write rendered envData file"))
		return fmt.Errorf("writing env data lib %s: %w", e.renderedDataLibFilePath, err)
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
	if err := yaml.Unmarshal(envDataYaml, &envDataStruct); err != nil {
		log.Error().Err(err).Msg(e.Msg("Unable to unmarshal environment data yaml"))
		return fmt.Errorf("unmarshalling environment data yaml: %w", err)
	}

	e.argoCDEnabled = envDataStruct.ArgoCD.Enabled

	for _, app := range envDataStruct.Environment.Applications {
		proto := app.Proto
		if proto == "" {
			log.Error().Interface("app", app).Msg(e.Msg("Application prototype is not set"))
			continue
		}

		name := app.Name
		if name == "" {
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
	currentPath := e.cfg.RootDir
	envRelDir := strings.TrimPrefix(e.Dir, e.cfg.RootDir+string(filepath.Separator))
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

// Msg formats a log message with the environment ID as context.
func (e *Environment) Msg(msg string) string {
	formattedMessage := fmt.Sprintf(EnvLogFormat, e.ID, initStepName, msg)
	return formattedMessage
}

func (e *Environment) ytt(purpose string, paths []string, args ...string) (CmdResult, error) {
	return e.yttS(purpose, paths, nil, args...)
}

func (e *Environment) yttS(purpose string, paths []string, stdin io.Reader, args ...string) (CmdResult, error) {
	paths = concatenate(e.extraYttPaths, paths)
	return runYttWithFilesAndStdin("ytt", paths, stdin, func(name string, err error, stderr string, args []string) {
		cmd := msgRunCmd(purpose, name, args)
		if err != nil {
			log.Error().Msg(e.Msg(cmd))
			log.Error().Msg(e.Msg(stderr))
		} else {
			log.Debug().Msg(e.Msg(cmd))
		}
	}, args...)
}

// GetApplicationNames returns the names of all applications in this environment.
func (e *Environment) GetApplicationNames() []string {
	var appNames []string
	for _, app := range e.Applications {
		appNames = append(appNames, app.Name)
	}
	return appNames
}

func (e *Environment) getYttLibAPIDir() string {
	return filepath.Join(e.cfg.RootDir, e.cfg.ServiceDirName, e.Dir, e.cfg.YttLibAPIDir)
}
