package myks

import (
	"path/filepath"

	"github.com/rs/zerolog/log"
)

type GlobalYtt struct {
	ident    string
	app      *Application
	additive bool
}

func (g *GlobalYtt) Ident() string {
	return g.ident
}

func (g *GlobalYtt) IsAdditive() bool {
	return g.additive
}

func (g *GlobalYtt) Render(previousStepFile string) (string, error) {
	var yttFiles []string

	yttFiles = append(yttFiles, g.app.yttDataFiles...)

	if previousStepFile != "" {
		yttFiles = append(yttFiles, previousStepFile)
	}

	yttFiles = append(yttFiles, g.app.e.collectBySubpath(filepath.Join("_env", g.app.e.g.YttStepDirName))...)

	if len(yttFiles) == 0 {
		log.Debug().Msg(g.app.Msg(globalYttStepName, "No ytt files found"))
		return "", nil
	}

	yttOutput, err := g.app.ytt(globalYttStepName, "render global ytt directory", yttFiles)
	if err != nil {
		return "", err
	}

	if yttOutput.Stdout == "" {
		log.Warn().Msg(g.app.Msg(globalYttStepName, "Empty ytt output"))
		return "", nil
	}

	log.Debug().Msg(g.app.Msg(globalYttStepName, "Global ytt applied"))

	return yttOutput.Stdout, nil
}
