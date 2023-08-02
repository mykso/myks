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

	yttFiles = append(yttFiles, g.app.e.collectBySubpath(filepath.Join("_env", g.app.e.g.YttPkgStepDirName))...)

	if len(yttFiles) == 0 {
		log.Debug().Str("app", g.app.Name).Msg("No ytt files found")
		return "", nil
	}

	log.Debug().Str("step", "global-ytt").Strs("files", yttFiles).Str("app", g.app.Name).Msg("Collected ytt files")

	yttOutput, err := g.app.e.g.ytt(yttFiles)
	if err != nil {
		log.Warn().Err(err).Str("app", g.app.Name).Msg("Unable to render ytt files")
		return "", err
	}

	if yttOutput.Stdout == "" {
		log.Warn().Str("app", g.app.Name).Msg("Empty ytt output")
		return "", nil
	}

	return yttOutput.Stdout, nil
}
