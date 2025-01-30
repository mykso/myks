package myks

import (
	"bytes"
	_ "embed"
	"path/filepath"
	"text/template"

	"github.com/rs/zerolog/log"
)

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

func (e *Environment) renderArgoCD() (err error) {
	if !e.argoCDEnabled {
		log.Debug().Msg(e.Msg("ArgoCD is disabled"))
		return
	}

	// 0. Global data values schema and library files are added later in the a.yttS call
	// 1. Collection of environment main data values and schemas
	yttFiles := e.collectBySubpath(e.g.EnvironmentDataFileName)
	// 2. Collection of environment argocd-specific data values and schemas, and overlays
	yttFiles = append(yttFiles, e.collectBySubpath(filepath.Join(e.g.EnvsDir, e.g.ArgoCDDataDirName))...)

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
	return filepath.Join(e.g.RootDir, e.g.RenderedArgoDir, e.ID)
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
	// 1. Dynamic ArgoCD default values
	yttFiles := []string{defaultsPath}
	// 2. Collection of application main data values and schemas
	yttFiles = append(yttFiles, a.yttDataFiles...)
	// 3. Use argocd-specific data values, schemas, and overlays from the prototype
	prototypeArgoCDDir := filepath.Join(a.Prototype, a.e.g.ArgoCDDataDirName)
	if ok, errExists := isExist(prototypeArgoCDDir); errExists != nil {
		return errExists
	} else if ok {
		yttFiles = append(yttFiles, prototypeArgoCDDir)
	}
	// 4. Collection of environment argocd-specific data values and schemas, and overlays
	yttFiles = append(yttFiles, a.e.collectBySubpath(filepath.Join(a.e.g.EnvsDir, a.e.g.ArgoCDDataDirName))...)
	// 5. Collection of application argocd-specific data values and schemas, and overlays
	yttFiles = append(yttFiles, a.e.collectBySubpath(filepath.Join(a.e.g.AppsDir, a.Name, a.e.g.ArgoCDDataDirName))...)

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

func (a *Application) argoCDPrepareDefaults() (filename string, err error) {
	const name = "argocd_defaults.ytt.yaml"

	tmpl, err := template.New(name).Parse(argocdDataValuesSchema)
	if err != nil {
		return
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
	err = tmpl.Execute(buf, data)
	if err != nil {
		return
	}

	err = a.writeServiceFile(name, buf.String())

	filename = a.expandServicePath(name)

	return
}

func (a *Application) getArgoCDDestinationDir() string {
	return filepath.Join(a.e.g.RootDir, a.e.g.RenderedArgoDir, a.e.ID)
}

func getArgoCDEnvFileName(envName string) string {
	return "env-" + envName + ".yaml"
}

func getArgoCDAppFileName(appName string) string {
	return "app-" + appName + ".yaml"
}
