package myks

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

type YttPkg struct {
	ident    string
	app      *Application
	additive bool
}

func (y *YttPkg) IsAdditive() bool {
	return y.additive
}

func (y *YttPkg) Ident() string {
	return y.ident
}

func (y *YttPkg) Render(_ string) (string, error) {
	yttPkgSubDirs, err := y.getYttPkgSubDirs()
	if err != nil {
		return "", err
	}

	if len(yttPkgSubDirs) == 0 {
		log.Debug().Msg(y.app.Msg(y.getStepName(), "No ytt packages found"))
		return "", nil
	}

	var outputs []string

	for _, pkgDir := range yttPkgSubDirs {
		out, err := y.renderPkg(pkgDir)
		if err != nil {
			return "", err
		}
		if out != "" {
			outputs = append(outputs, out)
		}
	}

	log.Info().Msg(y.app.Msg(y.getStepName(), "Ytt package rendered"))

	return strings.Join(outputs, "---\n"), nil
}

func (y *YttPkg) getYttPkgSubDirs() ([]string, error) {
	vendorYttPkgDir := y.app.expandVendorPath(y.app.cfg.YttPkgStepDirName)
	ok, err := isExist(vendorYttPkgDir)
	if err != nil {
		return nil, fmt.Errorf("error: %w", err)
	}
	if !ok {
		return nil, nil
	}

	vendorYttPkgFiles, err := readDirDereferenceLinks(vendorYttPkgDir)
	if err != nil {
		return nil, fmt.Errorf("error: %w", err)
	}

	var yttPkgSubDirs []string
	for _, file := range vendorYttPkgFiles {
		ok, err := isDir(file)
		if err != nil {
			return nil, fmt.Errorf("error: %w", err)
		}
		if ok {
			yttPkgSubDirs = append(yttPkgSubDirs, file)
		} else {
			log.Warn().Str("file", file).Msg(y.app.Msg(y.getStepName(), "Ignoring non-directory file in ytt package directory"))
		}
	}
	return yttPkgSubDirs, nil
}

func (y *YttPkg) renderPkg(pkgDir string) (string, error) {
	pkgName := filepath.Base(pkgDir)

	var yttFiles []string
	for _, yttFile := range y.app.yttPkgDirs {
		yttFiles = append(yttFiles, filepath.Join(pkgDir, yttFile))
	}
	if len(yttFiles) == 0 {
		yttFiles = []string{pkgDir}
	}

	var yttArgs []string
	pkgValuesFile, err := y.app.prepareValuesFile(y.app.cfg.YttPkgStepDirName, pkgName)
	if err != nil {
		log.Error().Err(err).Msg(y.app.Msg(y.getStepName(), "Unable to prepare a values file for the ytt package"))
		return "", fmt.Errorf("error: %w", err)
	}
	if pkgValuesFile != "" {
		yttArgs = []string{"--data-values-file=" + pkgValuesFile}
	}

	res, err := runYttWithFilesAndStdin(y.getStepName(), yttFiles, nil, y.app.cfg.Metrics, func(name string, err error, stderr string, args []string) {
		purpose := y.getStepName() + " render step"
		cmd := msgRunCmd(purpose, name, args)
		if err != nil {
			log.Error().Msg(cmd)
			log.Error().Msg(stderr)
		} else {
			log.Debug().Msg(cmd)
		}
	}, yttArgs...)
	if err != nil {
		return "", fmt.Errorf("error: %w", err)
	}

	if res.Stdout == "" {
		log.Warn().Str("pkgName", pkgName).Msg(y.app.Msg(y.getStepName(), "No ytt package output"))
		return "", nil
	}

	return res.Stdout, nil
}

func (y *YttPkg) getStepName() string {
	return fmt.Sprintf("%s-%s", renderStepName, y.Ident())
}
