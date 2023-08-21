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
var argocd_appproject_template []byte

//go:embed templates/argocd/application.ytt.yaml
var argocd_application_template []byte

const argocd_data_values_schema = `
#@data/values
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
	// 1. ArgoCD data value schema
	yttFiles := []string{}
	// 2. Collection of environment main data values and schemas
	yttFiles = append(yttFiles, e.collectBySubpath(e.g.EnvironmentDataFileName)...)
	// 3. Collection of environment argocd-specific data values and schemas, and overlays
	yttFiles = append(yttFiles, e.collectBySubpath(filepath.Join("_env", e.g.ArgoCDDataDirName))...)

	res, err := e.yttS(
		"create ArgoCD project yaml",
		yttFiles,
		bytes.NewReader(argocd_appproject_template),
	)
	if err != nil {
		log.Error().Err(err).Str("stdout", res.Stdout).Str("stderr", res.Stderr).Msg(e.Msg("failed to render ArgoCD project yaml"))
		return
	}

	argoDestinationPath := filepath.Join(e.getArgoCDDestinationDir(), "env-"+e.Id+".yaml")
	err = writeFile(argoDestinationPath, []byte(res.Stdout))
	if err != nil {
		return
	}

	return
}

func (e *Environment) getArgoCDDestinationDir() string {
	return filepath.Join(e.g.RootDir, e.g.RenderedDir, "argocd", e.Id)
}

func (a *Application) renderArgoCD() (err error) {
	if !a.argoCDEnabled {
		log.Debug().Msg(a.Msg(ArgoCDStepName, "ArgoCD is disabled"))
		return
	}

	defaultsPath, err := a.argoCDPrepareDefaults()
	if err != nil {
		return
	}

	// 0. Global data values schema and library files are added later in the a.yttS call
	// 1. ArgoCD data schema
	// 2. Dynamic ArgoCD default values
	yttFiles := []string{defaultsPath}
	// 2. Collection of application main data values and schemas
	yttFiles = append(yttFiles, a.yttDataFiles...)
	// 3. Collection of environment argocd-specific data values and schemas, and overlays
	yttFiles = append(yttFiles, a.e.collectBySubpath(filepath.Join("_env", a.e.g.ArgoCDDataDirName))...)
	// 4. Collection of application argocd-specific data values and schemas, and overlays
	yttFiles = append(yttFiles, a.e.collectBySubpath(filepath.Join("_apps", a.Name, a.e.g.ArgoCDDataDirName))...)

	res, err := a.yttS(
		"argocd",
		"create ArgoCD application yaml",
		yttFiles,
		bytes.NewReader(argocd_application_template),
	)
	if err != nil {
		log.Error().Err(err).
			Str("stdout", res.Stdout).
			Str("stderr", res.Stderr).
			Msg(a.Msg("argocd", "failed to render ArgoCD Application yaml"))
		return
	}

	argoDestinationPath := filepath.Join(a.getArgoCDDestinationDir(), "app-"+a.Name+".yaml")
	err = writeFile(argoDestinationPath, []byte(res.Stdout))
	if err != nil {
		return
	}

	return
}

func (a *Application) argoCDPrepareDefaults() (filename string, err error) {
	const name = "argocd_defaults.ytt.yaml"

	tmpl, err := template.New(name).Parse(argocd_data_values_schema)
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
		AppPath:        a.getDestinationDir(),
		RepoURL:        a.e.g.GitRepoUrl,
		TargetRevision: a.e.g.GitRepoBranch,
	}

	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, data)
	if err != nil {
		return
	}

	err = a.writeTempFile(name, buf.String())

	filename = a.expandTempPath(name)

	return
}

func (a *Application) getArgoCDDestinationDir() string {
	return filepath.Join(a.e.g.RootDir, a.e.g.RenderedDir, "argocd", a.e.Id)
}
