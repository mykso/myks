package kwhoosh

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

type Application struct {
	// Name of the application
	Name string
	// Application prototype directory
	Prototype string
	// Environment
	e *Environment
}

func NewApplication(e *Environment, name string, prototypeName string) (*Application, error) {
	if prototypeName == "" {
		prototypeName = name
	}

	prototype := filepath.Join(e.k.PrototypesDir, prototypeName)

	if _, err := os.Stat(prototype); err != nil {
		return nil, errors.New("Application prototype does not exist")
	}

	app := &Application{
		Name:      name,
		Prototype: prototype,
		e:         e,
	}

	return app, nil
}

func (a *Application) Init() error {
	// TODO: create application directory if it does not exist
	return nil
}

func (a *Application) Sync() error {
	if err := a.prepareSync(); err != nil {
		return err
	}
	return nil
}

func (a *Application) Render() error {
	return nil
}

func (a *Application) prepareSync() error {
	// Collect ytt arguments following the following steps:
	// 1. If exists, use the `apps/<prototype>/vendir` directory.
	// 2. If exists, for every level of environments use `<env>/_apps/<app>/vendir` directory.

	yttFiles := []string{}

	protoVendirDir := filepath.Join(a.Prototype, "vendir")
	if _, err := os.Stat(protoVendirDir); err == nil {
		yttFiles = append(yttFiles, protoVendirDir)
		log.Debug().Str("dir", protoVendirDir).Msg("Using prototype vendir directory")
	}

	appVendirDirs := a.e.collectBySubpath(filepath.Join("_apps", a.Name, "vendir"))
	yttFiles = append(yttFiles, appVendirDirs...)

	if len(yttFiles) == 0 {
		err := errors.New("No vendir configs found")
		log.Warn().Err(err).Str("app", a.Name).Msg("")
		return err
	}

	vendirConfig, err := runYttWithFiles(yttFiles)
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to render vendir config")
		return err
	}

	if vendirConfig.Stdout == "" {
		err = errors.New("Empty vendir config")
		log.Warn().Err(err).Msg("")
		return err
	}

	// TODO: rename `rendered` to `.kwhoosh` ?
	vendirConfigFilePath := filepath.Join(a.e.Dir, "_apps", a.Name, a.e.k.ServiceDirName, a.e.k.VendirConfigFileName)
	// Create directory if it does not exist
	err = os.MkdirAll(filepath.Dir(vendirConfigFilePath), 0o755)
	if err != nil {
		log.Warn().Err(err).Msg("Unable to create directory for vendir config file")
		return err
	}
	err = os.WriteFile(vendirConfigFilePath, []byte(vendirConfig.Stdout), 0o644)
	if err != nil {
		log.Warn().Err(err).Msg("Unable to write vendir config file")
		return err
	}
	log.Debug().Str("file", vendirConfigFilePath).Msg("Wrote vendir config file")

	return nil
}
