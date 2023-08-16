package myks

import (
	"bytes"
	_ "embed"
	"path/filepath"
	"text/template"

	"github.com/rs/zerolog/log"
)

const ArgoCDStepName = "argocd"

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
`

func (a *Application) renderArgoCD() (err error) {
	if !a.argoCDEnabled {
		log.Debug().Msg(a.Msg(ArgoCDStepName, "ArgoCD is disabled"))
		return
	}

	schemaFile, err := a.argoCDPrepareSchema()
	if err != nil {
		return
	}

	// 0. Global data values schema and library files are added later in the a.yttS call
	// 1. Dynamyc ArgoCD data values
	yttFiles := []string{schemaFile}
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
		return
	}

	filepath := filepath.Join(a.getArgoCDDestinationDir(), "app-"+a.Name+".yaml")
	err = writeFile(filepath, []byte(res.Stdout))
	if err != nil {
		return
	}

	return
}

func (a *Application) argoCDPrepareSchema() (filename string, err error) {
	const name = "argocd_data_schema.ytt.yaml"

	tmpl, err := template.New(name).Parse(string(argocd_data_values_schema))
	if err != nil {
		return
	}

	type Data struct {
		AppName string
		AppPath string
	}

	data := Data{
		AppName: a.Name,
		AppPath: a.getDestinationDir(),
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
