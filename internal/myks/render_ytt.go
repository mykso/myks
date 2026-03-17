package myks

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mykso/myks/internal/locker"
	"github.com/rs/zerolog/log"
)

type Ytt struct {
	additive bool
	app      *Application
	ident    string
	locker   *locker.Locker
}

func NewYttRenderer(app *Application, lock *locker.Locker) *Ytt {
	return &Ytt{
		additive: false,
		app:      app,
		ident:    "ytt",
		locker:   lock,
	}
}

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

func (y *Ytt) Render(previousStepFile string) (string, error) {
	var yttFiles []string

	// add environment, prototype, and application data files
	yttFiles = append(yttFiles, y.app.yttDataFiles...)

	// if yamls were rendered during the last step, we might want to modify them during this step
	// therefore, add them as well
	if previousStepFile != "" {
		yttFiles = append(yttFiles, previousStepFile)
	}

	// we might have vendored some yamls or json files that we want to transform during this step
	// therefore, add them as well
	vendorYttDir := y.app.expandVendorPath(y.app.cfg.YttStepDirName)
	if ok, err := isExist(vendorYttDir); err != nil {
		return "", err
	} else if ok {
		// symlinks to directories are not followed by ytt, so we need to dereference them
		vendorYttFiles, err := readDirDereferenceLinks(vendorYttDir)
		if err != nil {
			return "", err
		}
		yttFiles = append(yttFiles, vendorYttFiles...)
	} else {
		log.Debug().Msg(y.app.Msg(y.getStepName(), "No vendor ytt directory found"))
	}

	// we obviously want to add the ytt files from the prototype dir
	prototypeYttDir := filepath.Join(y.app.Prototype, y.app.cfg.YttStepDirName)
	if ok, err := isExist(prototypeYttDir); err != nil {
		return "", err
	} else if ok {
		yttFiles = append(yttFiles, prototypeYttDir)
	}

	// we might have some prototype overwrites in the environment group folders.
	// let's iterate over the environment directory structure and add them
	// these should follow the structure and naming using in the prototypes directory
	yttFiles = append(yttFiles, collectBySubpath(y.app.cfg.RootDir, y.app.e.Dir, filepath.Join(y.app.cfg.PrototypeOverrideDir, y.app.prototypeDirName(), y.app.cfg.YttStepDirName))...)

	// finally, lets add the ytt directories from the application directory and the environment group folders
	yttFiles = append(yttFiles, collectBySubpath(y.app.cfg.RootDir, y.app.e.Dir, filepath.Join(y.app.cfg.AppsDir, y.app.Name, y.app.cfg.YttStepDirName))...)

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
