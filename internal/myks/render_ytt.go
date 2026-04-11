package myks

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/mykso/myks/internal/locker"
)

type Ytt struct {
	additive bool
	app      *Application
	ident    string
	locker   *locker.Locker
}

// NewYttRenderer creates a Ytt renderer that applies application-level ytt overlays.
func NewYttRenderer(app *Application, lock *locker.Locker) *Ytt {
	return &Ytt{
		additive: false,
		app:      app,
		ident:    "ytt",
		locker:   lock,
	}
}

// AcquireLock acquires a read lock on the ytt vendor directory for this application.
func (y *Ytt) AcquireLock() (func(), error) {
	return y.app.AcquireRenderLock(y.locker, func(path string) bool {
		return strings.HasPrefix(path, y.app.cfg.YttStepDirName+"/")
	}, false)
}

func (y *Ytt) IsAdditive() bool {
	return y.additive
}

func (y *Ytt) Ident() string {
	return y.ident
}

// yttSourceFiles returns the ytt-specific source files and directories for this application.
// It searches in (order preserved for ytt overlay precedence):
//   - vendored ytt files: .myks/<env>/_apps/<app>/vendor/ytt/ (dereferenced symlinks)
//   - prototypes/<prototype>/ytt/
//   - envs/**/_proto/<prototype>/ytt/ (at each env hierarchy level)
//   - envs/**/_apps/<app>/ytt/ (at each env hierarchy level)
//
// Note: vendor files only exist after sync; they are omitted if not present.
// Used by both ytt render and inspect.
func (a *Application) yttSourceFiles() ([]string, error) {
	var files []string

	// vendored ytt files (only present after sync)
	vendorYttDir := a.expandVendorPath(a.cfg.YttStepDirName)
	if ok, err := isExist(vendorYttDir); err != nil {
		return nil, err
	} else if ok {
		vendorYttFiles, err := readDirDereferenceLinks(vendorYttDir)
		if err != nil {
			return nil, err
		}
		files = append(files, vendorYttFiles...)
	}

	// prototype ytt directory
	prototypeYttDir := filepath.Join(a.Prototype, a.cfg.YttStepDirName)
	if ok, err := isExist(prototypeYttDir); err != nil {
		return nil, err
	} else if ok {
		files = append(files, prototypeYttDir)
	}

	// prototype override ytt dirs at each env hierarchy level
	files = append(files, collectBySubpath(a.cfg.RootDir, a.e.Dir, filepath.Join(a.cfg.PrototypeOverrideDir, a.prototypeDirName(), a.cfg.YttStepDirName))...)

	// application ytt dirs at each env hierarchy level
	files = append(files, collectBySubpath(a.cfg.RootDir, a.e.Dir, filepath.Join(a.cfg.AppsDir, a.Name, a.cfg.YttStepDirName))...)

	return files, nil
}

// Render applies ytt overlays from the prototype, env hierarchy, and app directories
// to the accumulated YAML from previous steps.
func (y *Ytt) Render(previousStepFile string) (string, error) {
	// add environment, prototype, and application data files
	yttFiles := append([]string{}, y.app.yttDataFiles...)

	// if yamls were rendered during the last step, we might want to modify them during this step
	if previousStepFile != "" {
		yttFiles = append(yttFiles, previousStepFile)
	}

	sourceFiles, err := y.app.yttSourceFiles()
	if err != nil {
		return "", err
	}
	yttFiles = append(yttFiles, sourceFiles...)

	if len(yttFiles) == 0 {
		log.Debug().Msg(y.app.Msg(y.getStepName(), "No local ytt directory found"))
		return "", nil
	}

	res, err := y.app.ytt(y.getStepName(), "render local ytt", yttFiles)
	if err != nil {
		return "", err
	}

	if res.Stdout == "" {
		log.Warn().Msg(y.app.Msg(y.getStepName(), "Empty ytt output"))
		return "", nil
	}

	log.Info().Msg(y.app.Msg(y.getStepName(), "Local ytt rendered"))

	return res.Stdout, nil
}

func (y *Ytt) getStepName() string {
	return fmt.Sprintf("%s-%s", renderStepName, y.Ident())
}
