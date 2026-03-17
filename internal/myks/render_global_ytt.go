package myks

import (
	"path/filepath"

	"github.com/mykso/myks/internal/locker"
	"github.com/rs/zerolog/log"
)

type GlobalYtt struct {
	additive bool
	app      *Application
	ident    string
	locker   *locker.Locker
}

// NewGlobalYttRenderer creates a GlobalYtt renderer that applies environment-level ytt overlays.
func NewGlobalYttRenderer(app *Application, lock *locker.Locker) *GlobalYtt {
	return &GlobalYtt{
		additive: false,
		app:      app,
		ident:    globalYttStepName,
		locker:   lock,
	}
}

// AcquireLock is a no-op for GlobalYtt since it does not read from vendored sources.
func (g *GlobalYtt) AcquireLock() (func(), error) {
	// No lock needed for global ytt since it doesn't read any sources.
	return func() {}, nil
}

func (g *GlobalYtt) Ident() string {
	return g.ident
}

func (g *GlobalYtt) IsAdditive() bool {
	return g.additive
}

func (g *GlobalYtt) Render(previousStepFile string) (string, error) {
	yttFiles := make([]string, len(g.app.yttDataFiles))
	copy(yttFiles, g.app.yttDataFiles)

	if previousStepFile != "" {
		yttFiles = append(yttFiles, previousStepFile)
	}

	// Global or environment-specific ytt files.
	// By default, located in `envs/<env>/_env/ytt`.
	globalYttFiles := g.app.e.collectBySubpath(filepath.Join(g.app.cfg.EnvsDir, g.app.cfg.YttStepDirName))
	yttFiles = append(yttFiles, globalYttFiles...)

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
