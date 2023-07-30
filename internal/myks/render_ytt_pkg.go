package myks

import (
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

func (a *Application) runYttPkg() (string, error) {
	yttPkgRootDir, err := a.getVendoredDir(a.e.g.YttPkgStepDirName)
	if err != nil {
		log.Err(err).Str("app", a.Name).Msg("Unable to get ytt package dir")
		return "", err
	}
	yttPkgSubDirs := getSubDirs(yttPkgRootDir)
	if len(yttPkgSubDirs) == 0 {
		log.Debug().Str("app", a.Name).Msg("No packages to process")
		return "", nil
	}

	var outputs []string

	for _, pkgDir := range yttPkgSubDirs {
		pkgName := filepath.Base(pkgDir)
		var pkgValuesFile string
		if pkgValuesFile, err = a.prepareValuesFile("ytt-pkg", pkgName); err != nil {
			log.Warn().Err(err).Str("app", a.Name).Msg("Unable to prepare vendir packages value files")
			return "", err
		}

		var yttFiles []string
		for _, yttFile := range a.yttPkgDirs {
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
			log.Error().Err(err).Str("app", a.Name).Str("stdout", res.Stdout).Str("stderr", res.Stderr).Msg("Unable to run ytt")
			return "", err
		}

		if res.Stdout == "" {
			log.Warn().Str("app", a.Name).Str("pkgName", pkgName).Msg("No ytt package output")
			continue
		}

		outputs = append(outputs, res.Stdout)
	}

	return strings.Join(outputs, "---\n"), nil
}
