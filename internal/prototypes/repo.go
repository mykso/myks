package prototypes

import (
	"fmt"
	"strings"
)

type Repo string

const (
	Git       Repo = "git"
	HelmChart Repo = "helmchart"
)

func (r Repo) String() string {
	return string(r)
}

func (r *Repo) Set(s string) error {
	sl := strings.ToLower(s)
	repo := Repo(sl)
	switch sl {
	case "git", "helmchart":
		*r = repo
	default:
		return fmt.Errorf("unknown repo type: %s", s)
	}
	return nil
}

func (r Repo) Type() string {
	return "repo"
}
