package myks

import (
	"bytes"
	_ "embed"
	"path/filepath"
	"text/template"
)

//go:embed templates/argocd_application.ytt.yaml
var argocd_application_template []byte

const argocd_data_values_schema = `
#@data/values-schema
---
argocd:
  app:
    name: "{{ .AppName }}"
  source:
    path: "{{ .AppPath }}"
`

func (a *Application) renderArgoCD() (err error) {
	schemaFile, err := a.argoCDPrepareSchema()
	if err != nil {
		return
	}

	res, err := a.yttS(
		"argocd",
		"create ArgoCD application yaml",
		[]string{schemaFile},
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
