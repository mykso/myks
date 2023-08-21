package myks

import (
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
	yttPkgRootDir, err := y.app.getVendoredDir(y.app.e.g.YttPkgStepDirName)
	if err != nil {
		log.Error().Err(err).Msg(y.app.Msg(yttPkgStepName, "Unable to get ytt package dir"))
		return "", err
	}

	yttPkgSubDirs, err := getSubDirs(yttPkgRootDir)
	if err != nil {
		log.Error().Err(err).Msg(y.app.Msg(yttPkgStepName, "Unable to get ytt package sub dirs"))
		return "", err
	}
	if len(yttPkgSubDirs) == 0 {
		log.Debug().Msg(y.app.Msg(yttPkgStepName, "No ytt packages found"))
		return "", nil
	}

	var outputs []string

	for _, pkgDir := range yttPkgSubDirs {
		pkgName := filepath.Base(pkgDir)

		var yttFiles []string
		for _, yttFile := range y.app.yttPkgDirs {
			yttFiles = append(yttFiles, filepath.Join(pkgDir, yttFile))
		}
		if len(yttFiles) == 0 {
			yttFiles = []string{pkgDir}
		}

		var yttArgs []string
		if pkgValuesFile, err := y.app.prepareValuesFile("ytt-pkg", pkgName); err != nil {
			log.Error().Err(err).Msg(y.app.Msg(yttPkgStepName, "Unable to prepare a values file for the ytt package"))
			return "", err
		} else if pkgValuesFile != "" {
			yttArgs = []string{"--data-values-file=" + pkgValuesFile}
		}

		res, err := runYttWithFilesAndStdin(yttFiles, nil, func(name string, args []string) {
			// make this copy-n-pastable
			log.Debug().Msg(msgRunCmd("ytt-pkg render step", name, args))
		}, yttArgs...)
		if err != nil {
			log.Error().Err(err).Str("stdout", res.Stdout).Str("stderr", res.Stderr).Msg(y.app.Msg(yttPkgStepName, "Unable to run ytt"))
			return "", err
		}

		if res.Stdout == "" {
			log.Warn().Str("pkgName", pkgName).Msg(y.app.Msg(yttPkgStepName, "No ytt package output"))
			continue
		}

		outputs = append(outputs, res.Stdout)
	}

	log.Info().Msg(y.app.Msg(helmStepName, "Ytt package rendered"))

	return strings.Join(outputs, "---\n"), nil
}
