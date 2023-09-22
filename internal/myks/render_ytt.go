package myks

import (
	"errors"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

type Ytt struct {
	ident    string
	app      *Application
	additive bool
}

func (y *Ytt) IsAdditive() bool {
	return y.additive
}

func (y *Ytt) Ident() string {
	return y.ident
}

func (y *Ytt) Render(previousStepFile string) (string, error) {
	var yttFiles []string
	yttFiles = append(yttFiles, y.app.yttDataFiles...)

	if previousStepFile != "" {
		yttFiles = append(yttFiles, previousStepFile)
	}

	vendorYttDir := y.app.expandPath(filepath.Join(y.app.e.g.VendorDirName, y.app.e.g.YttStepDirName))
	if ok, err := isExist(vendorYttDir); err != nil {
		return "", err
	} else if ok {
		yttFiles = append(yttFiles, vendorYttDir)
	}

	prototypeYttDir := filepath.Join(y.app.Prototype, y.app.e.g.YttStepDirName)
	if ok, err := isExist(prototypeYttDir); err != nil {
		return "", err
	} else if ok {
		yttFiles = append(yttFiles, prototypeYttDir)
	}

	yttFiles = append(yttFiles, y.app.e.collectBySubpath(filepath.Join("_apps", y.app.Name, y.app.e.g.YttStepDirName))...)

	if len(yttFiles) == 0 {
		log.Debug().Msg(y.app.Msg(yttStepName, "No local ytt directory found"))
		return "", nil
	}

	res, err := y.app.ytt(yttStepName, "render local ytt", yttFiles)
	if err != nil {
		log.Error().Msg(y.app.Msg(yttStepName, res.Stderr))
		return "", err
	}

	if res.Stdout == "" {
		return "", errors.New("empty ytt output")
	}

	log.Info().Msg(y.app.Msg(yttStepName, "Local ytt rendered"))

	return res.Stdout, nil
}
