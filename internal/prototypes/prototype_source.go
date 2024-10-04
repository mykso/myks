package prototypes

import (
	"errors"
)

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

type BumpResult int

const (
	Failed BumpResult = iota
	Bumped
	UpToDate
	Unsupported
)

func (p *Prototype) Bump(s Source) (BumpResult, error) {
	change, err := s.bump()
	if err != nil {
		return Failed, err
	}
	p.AddSource(s)
	return change, err
}

func (s *Source) bump() (BumpResult, error) {
	switch s.Repo {
	case HelmChart:
		return s.bumpHelm()
	default:
		return Unsupported, nil
	}
}

func (s *Source) bumpHelm() (BumpResult, error) {
	h := &HelmClient{}
	repo, err := h.RepoName(s.Url)
	if err != nil {
		return Failed, err
	}
	if repo == "" {
		return Failed, errors.New("repository not found in helm repos. Consider adding it to helm")

	}

	version, err := h.ChartVersion(s.Url, s.Name)
	if err != nil {
		return Failed, err
	}
	if len(version) == 0 {
		return Failed, errors.New("no versions found")
	}
	if version[0] == s.Version {
		return UpToDate, nil
	}
	s.Version = version[0]
	return Bumped, nil
}
