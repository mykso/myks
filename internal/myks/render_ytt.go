package myks

import (
	"errors"
	"os"
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
	if _, err := os.Stat(vendorYttDir); err == nil {
		yttFiles = append(yttFiles, vendorYttDir)
	}

	prototypeYttDir := filepath.Join(y.app.Prototype, y.app.e.g.YttStepDirName)
	if _, err := os.Stat(prototypeYttDir); err == nil {
		yttFiles = append(yttFiles, prototypeYttDir)
	}

	yttFiles = append(yttFiles, y.app.e.collectBySubpath(filepath.Join("_apps", y.app.Name, y.app.e.g.YttStepDirName))...)

	if len(yttFiles) == 0 {
		log.Debug().Msg(y.app.Msg(yttStepName, "No local ytt directory found"))
		return "", nil
	}

	yamlOutput, err := y.app.ytt(yttStepName, "render local ytt", yttFiles)
	if err != nil {
		return "", err
	}

	if yamlOutput.Stdout == "" {
		return "", errors.New("empty ytt output")
	}

	log.Info().Msg(y.app.Msg(yttStepName, "Local ytt rendered"))

	return yamlOutput.Stdout, nil
}
