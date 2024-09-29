package prototypes

type Kind string

const (
	Ytt    Kind = "ytt"
	Helm   Kind = "helm"
	Static Kind = "static"
	YttPkg Kind = "ytt-pkg"
)

type Source struct {
	Name         string   `yaml:"name"`
	Kind         Kind     `yaml:"kind"`
	Repo         Repo     `yaml:"repo"`
	Url          string   `yaml:"url"`
	Version      string   `yaml:"version"`
	NewRootPath  string   `yaml:"newRootPath,omitempty"`
	IncludePaths []string `yaml:"includePaths,omitempty"`
}

func (p *Prototype) GetSource(name string) (Source, bool) {
	for _, proto := range p.Sources {
		if proto.Name == name {
			return proto, true
		}
	}
	return Source{}, false
}

func (p *Prototype) AddSource(proto Source) {
	for i := range p.Sources {
		if p.Sources[i].Name == proto.Name {
			p.Sources[i] = proto
			return
		}
	}
	p.Sources = append(p.Sources, proto)
}

func (p *Prototype) DelSource(name string) {
	for i, proto := range p.Sources {
		if proto.Name == name {
			p.Sources = append(p.Sources[:i], p.Sources[i+1:]...)
			return
		}
	}
}
