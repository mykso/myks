package prototypes

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	_ "embed"

	"github.com/mykso/myks/internal/myks"
	"gopkg.in/yaml.v3"
)

//go:embed assets
var assets embed.FS

const (
	baseYttFile        = "assets/base.ytt.yaml"
	dataValuesTemplate = "assets/data-values.ytt.yaml.template"
)

type Kind string

const (
	Ytt    Kind = "ytt"
	Helm   Kind = "helm"
	Static Kind = "static"
	YttPkg Kind = "ytt-pkg"
)

type Repo string

const (
	Git       Repo = "git"
	HelmChart Repo = "helmChart"
)

type Prototypes struct {
	file       string
	Prototypes []Source `yaml:"prototypes"`
}

type Source struct {
	Name         string   `yaml:"name"`
	Kind         Kind     `yaml:"kind"`
	Repo         Repo     `yaml:"repo"`
	Url          string   `yaml:"url"`
	Version      string   `yaml:"version"`
	NewRootPath  string   `yaml:"newRootPath,omitempty"`
	IncludePaths []string `yaml:"includePaths,omitempty"`
}

func Create(m *myks.Globe, name string) (Prototypes, error) {
	file := pathFromName(m, name)
	err := os.MkdirAll(filepath.Dir(file), os.ModePerm)
	if err != nil {
		return Prototypes{}, err
	}
	return Prototypes{
		file: file,
	}, nil
}

func Load(m *myks.Globe, protoName string) (Prototypes, error) {
	filepath := pathFromName(m, protoName)
	protos := Prototypes{}
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

func pathFromName(m *myks.Globe, name string) string {
	file := name
	if !strings.HasPrefix(file, m.PrototypesDir) {
		file = filepath.Join(m.PrototypesDir, file)
	}
	if !strings.HasSuffix("vendir/vendir-data.ytt.yaml", file) {
		file = filepath.Join(file, "vendir/vendir-data.ytt.yaml")
	}
	return file
}

func (p *Prototypes) GetSource(name string) (Source, bool) {
	for _, proto := range p.Prototypes {
		if proto.Name == name {
			return proto, true
		}
	}
	return Source{}, false
}

func (p *Prototypes) AddSource(proto Source) {
	for i := range p.Prototypes {
		if p.Prototypes[i].Name == proto.Name {
			p.Prototypes[i] = proto
			return
		}
	}
	p.Prototypes = append(p.Prototypes, proto)
}

func (p *Prototypes) Save() error {
	dataValuesYaml, err := yaml.Marshal(p)
	if err != nil {
		return err
	}

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

	err = os.WriteFile(p.file, buffer.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("failed to write data-values file to %s: %w", p.file, err)
	}

	baseYtt, err := assets.ReadFile(baseYttFile)
	if err != nil {
		return fmt.Errorf("failed to read base ytt file: %w. This is a bug in myks", err)
	}
	dest := filepath.Dir(p.file)
	dest = filepath.Join(dest, "base.ytt.yaml")
	err = os.WriteFile(dest, baseYtt, 0644)
	if err != nil {
		return fmt.Errorf("failed to write base ytt file to %s: %w", dest, err)
	}

	return nil
}
