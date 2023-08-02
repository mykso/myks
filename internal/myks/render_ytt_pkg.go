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
		log.Err(err).Str("app", y.app.Name).Msg("Unable to get ytt package dir")
		return "", err
	}
	yttPkgSubDirs := getSubDirs(yttPkgRootDir)
	if len(yttPkgSubDirs) == 0 {
		log.Debug().Str("app", y.app.Name).Msg("No packages to process")
		return "", nil
	}

	var outputs []string

	for _, pkgDir := range yttPkgSubDirs {
		pkgName := filepath.Base(pkgDir)
		var pkgValuesFile string
		if pkgValuesFile, err = y.app.prepareValuesFile("ytt-pkg", pkgName); err != nil {
			log.Warn().Err(err).Str("app", y.app.Name).Msg("Unable to prepare vendir packages value files")
			return "", err
		}

		var yttFiles []string
		for _, yttFile := range y.app.yttPkgDirs {
			yttFiles = append(yttFiles, filepath.Join(pkgDir, yttFile))
		}
		if len(yttFiles) == 0 {
			yttFiles = append(yttFiles, pkgDir)
		}

		var yttArgs []string
		if pkgValuesFile != "" {
			yttArgs = append(yttArgs, "--data-values-file="+pkgValuesFile)
		}

		res, err := runYttWithFilesAndStdin(yttFiles, nil, yttArgs...)
		if err != nil {
			log.Error().Err(err).Str("app", y.app.Name).Str("stdout", res.Stdout).Str("stderr", res.Stderr).Msg("Unable to run ytt")
			return "", err
		}

		if res.Stdout == "" {
			log.Warn().Str("app", y.app.Name).Str("pkgName", pkgName).Msg("No ytt package output")
			continue
		}

		outputs = append(outputs, res.Stdout)
	}

	return strings.Join(outputs, "---\n"), nil
}
