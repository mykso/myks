package prototypes

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/mykso/myks/internal/myks"
	"gopkg.in/yaml.v3"
)

//go:embed assets
var assets embed.FS

const (
	baseYttFile        = "assets/base.ytt.yaml"
	dataValuesTemplate = "assets/data-values.ytt.yaml.template"
)

type Prototype struct {
	file       string
	Prototypes []Source `yaml:"prototypes"`
}

func Create(g *myks.Globe, name string) (Prototype, error) {
	file := pathFromName(g, name)
	err := os.MkdirAll(filepath.Dir(file), os.ModePerm)
	if err != nil {
		return Prototype{}, err
	}
	return Prototype{
		file: file,
	}, nil
}

func Load(g *myks.Globe, protoName string) (Prototype, error) {
	filepath := pathFromName(g, protoName)
	protos := Prototype{}
	content, err := os.ReadFile(filepath)
	if err != nil {
		return protos, err
	}
	err = yaml.Unmarshal(content, &protos)
	if err != nil {
		return protos, err
	}
	protos.file = filepath
	return protos, nil
}

func Delete(g *myks.Globe, name string) error {
	filepath := pathFromName(g, name)
	return os.Remove(filepath)
}

func pathFromName(g *myks.Globe, name string) string {
	file := name
	if !strings.HasPrefix(file, g.PrototypesDir) {
		file = filepath.Join(g.PrototypesDir, file)
	}
	if !strings.HasSuffix("vendir/vendir-data.ytt.yaml", file) {
		file = filepath.Join(file, "vendir/vendir-data.ytt.yaml")
	}
	return file
}

func (p *Prototype) Save() error {
	buf := bytes.Buffer{}
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	err := enc.Encode(&p)
	if err != nil {
		return err
	}
	dataValuesYaml := buf.String()

	t, err := assets.ReadFile(dataValuesTemplate)
	if err != nil {
		return fmt.Errorf("failed to read data-values template: %w. This is a bug in myks", err)
	}
	tmpl, err := template.New("").Parse(string(t))
	if err != nil {
		return fmt.Errorf("failed to parse data-values template: %w. This is a bug in myks", err)
	}
	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, string(dataValuesYaml))
	if err != nil {
		return fmt.Errorf("failed to execute data-values template: %w", err)
	}

	err = os.WriteFile(p.file, buffer.Bytes(), 0o644)
	if err != nil {
		return fmt.Errorf("failed to write data-values file to %s: %w", p.file, err)
	}

	baseYtt, err := assets.ReadFile(baseYttFile)
	if err != nil {
		return fmt.Errorf("failed to read base ytt file: %w. This is a bug in myks", err)
	}
	dest := filepath.Dir(p.file)
	dest = filepath.Join(dest, "base.ytt.yaml")
	err = os.WriteFile(dest, baseYtt, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write base ytt file to %s: %w", dest, err)
	}

	return nil
}
