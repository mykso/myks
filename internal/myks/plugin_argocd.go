package myks

import (
	"bytes"
	_ "embed"
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/rs/zerolog/log"
)

// ArgoCDStepName is the step name identifier for the ArgoCD rendering plugin.
const ArgoCDStepName = "argocd"

//go:embed templates/argocd/environment.ytt.yaml
var argocdAppprojectTemplate []byte

//go:embed templates/argocd/application.ytt.yaml
var argocdApplicationTemplate []byte

const argocdDataValuesSchema = `
#@data/values-schema
---
argocd:
  app:
    name: "{{ .AppName }}"
    source:
      path: "{{ .AppPath }}"
      repoURL: "{{ .RepoURL }}"
      targetRevision: "{{ .TargetRevision }}"
`

// argoCDEnvSourceFiles returns the source files used to render the ArgoCD environment (AppProject) yaml.
// It includes the API library, env-data files, and environment-level argocd overlays.
// Used by both ArgoCD env render and inspect.
func (e *Environment) argoCDEnvSourceFiles() []string {
	files := []string{e.getYttLibAPIDir()}
	files = append(files, e.collectBySubpath(e.cfg.EnvironmentDataFileName)...)
	files = append(files, e.collectBySubpath(filepath.Join(e.cfg.EnvsDir, e.cfg.ArgoCDDataDirName))...)
	return files
}

func (e *Environment) renderArgoCD() error {
	if !e.argoCDEnabled {
		log.Debug().Msg(e.Msg("ArgoCD is disabled"))
		return nil
	}

	yttFiles := e.argoCDEnvSourceFiles()

	res, err := e.yttS(
		"create ArgoCD project yaml",
		yttFiles,
		bytes.NewReader(argocdAppprojectTemplate),
	)
	if err != nil {
		return err
	}
	if res.Stdout == "" {
		log.Info().Msg(e.Msg("ArgoCD environment (AppProject and repository Secret) yaml is empty"))
		return nil
	}

	argoDestinationPath := filepath.Join(e.getArgoCDDestinationDir(), getArgoCDEnvFileName(e.ID))
	return writeFile(argoDestinationPath, []byte(res.Stdout))
}

func (e *Environment) getArgoCDDestinationDir() string {
	return filepath.Join(e.cfg.RootDir, e.cfg.RenderedArgoDir, e.ID)
}

// argoCDAppSourceFiles returns the ArgoCD-specific source files for this application.
// It searches in:
//   - prototypes/<prototype>/argocd/
//   - envs/**/_env/argocd/ (at each env hierarchy level)
//   - envs/**/_apps/<app>/argocd/ (at each env hierarchy level)
//
// Note: the dynamically generated argocd_defaults.ytt.yaml is NOT included here
// as it is a runtime artifact, not a source file.
// Used by both ArgoCD app render and inspect.
func (a *Application) argoCDAppSourceFiles() ([]string, error) {
	var files []string

	prototypeArgoCDDir := filepath.Join(a.Prototype, a.cfg.ArgoCDDataDirName)
	if ok, err := isExist(prototypeArgoCDDir); err != nil {
		return nil, err
	} else if ok {
		files = append(files, prototypeArgoCDDir)
	}
	files = append(files, a.e.collectBySubpath(filepath.Join(a.cfg.EnvsDir, a.cfg.ArgoCDDataDirName))...)
	files = append(files, a.e.collectBySubpath(filepath.Join(a.cfg.AppsDir, a.Name, a.cfg.ArgoCDDataDirName))...)

	return files, nil
}

func (a *Application) renderArgoCD() error {
	if !a.argoCDEnabled {
		log.Debug().Msg(a.Msg(ArgoCDStepName, "ArgoCD is disabled"))
		return nil
	}

	defaultsPath, err := a.argoCDPrepareDefaults()
	if err != nil {
		return err
	}

	// 0. Global data values schema and library files are added later in the a.yttS call
	// 1. Dynamic ArgoCD default values (generated, not a source file)
	yttFiles := []string{defaultsPath}
	// 2. Collection of application main data values and schemas
	yttFiles = append(yttFiles, a.yttDataFiles...)
	// 3-5. ArgoCD-specific source files
	argoCDFiles, err := a.argoCDAppSourceFiles()
	if err != nil {
		return err
	}
	yttFiles = append(yttFiles, argoCDFiles...)

	res, err := a.yttS(
		"argocd",
		"create ArgoCD application yaml",
		yttFiles,
		bytes.NewReader(argocdApplicationTemplate),
	)
	if err != nil {
		log.Error().Err(err).
			Str("stdout", res.Stdout).
			Str("stderr", res.Stderr).
			Msg(a.Msg(ArgoCDStepName, "failed to render ArgoCD Application yaml"))
		return err
	}

	sortedBytes, err := sortYaml([]byte(res.Stdout))
	if err != nil {
		log.Error().Err(err).Msg(a.Msg(ArgoCDStepName, "failed to sort ArgoCD Application yaml"))
		return err
	}

	argoDestinationPath := filepath.Join(a.getArgoCDDestinationDir(), getArgoCDAppFileName(a.Name))
	return writeFile(argoDestinationPath, sortedBytes)
}

func (a *Application) argoCDPrepareDefaults() (string, error) {
	const name = "argocd_defaults.ytt.yaml"

	tmpl, err := template.New(name).Parse(argocdDataValuesSchema)
	if err != nil {
		log.Fatal().Err(err).Msg(a.Msg(ArgoCDStepName, "failed to parse ArgoCD data values schema template"))
		return "", fmt.Errorf("parsing ArgoCD data values schema template: %w", err)
	}

	type Data struct {
		AppName        string
		AppPath        string
		RepoURL        string
		TargetRevision string
	}

	data := Data{
		AppName:        a.Name,
		AppPath:        filepath.Join(a.e.g.GitPathPrefix, a.getDestinationDir()),
		RepoURL:        a.e.g.GitRepoURL,
		TargetRevision: a.e.g.GitRepoBranch,
	}

	buf := &bytes.Buffer{}
	if err = tmpl.Execute(buf, data); err != nil {
		return "", fmt.Errorf("executing ArgoCD data values schema template: %w", err)
	}

	if err = a.writeServiceFile(name, buf.String()); err != nil {
		return "", fmt.Errorf("writing ArgoCD defaults file: %w", err)
	}

	return a.expandServicePath(name), nil
}

func (a *Application) getArgoCDDestinationDir() string {
	return filepath.Join(a.cfg.RootDir, a.cfg.RenderedArgoDir, a.e.ID)
}

func getArgoCDEnvFileName(envName string) string {
	return "env-" + envName + ".yaml"
}

func getArgoCDAppFileName(appName string) string {
	return "app-" + appName + ".yaml"
}
