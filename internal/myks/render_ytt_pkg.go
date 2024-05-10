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
	var yttPkgSubDirs []string

	vendorYttPkgDir := y.app.expandVendorPath(y.app.e.g.YttPkgStepDirName)
	if ok, err := isExist(vendorYttPkgDir); err != nil {
		return "", err
	} else if ok {
		// symlinks to directories are not followed by ytt, so we need to dereference them
		vendorYttPkgFiles, err := readDirDereferenceLinks(vendorYttPkgDir)
		if err != nil {
			return "", err
		}
		// select only directories
		for _, file := range vendorYttPkgFiles {
			if ok, err := isDir(file); err != nil {
				return "", err
			} else if ok {
				yttPkgSubDirs = append(yttPkgSubDirs, file)
			} else {
				log.Warn().Str("file", file).Msg(y.app.Msg(y.getStepName(), "Ignoring non-directory file in ytt package directory"))
			}
		}
	}

	if len(yttPkgSubDirs) == 0 {
		log.Debug().Msg(y.app.Msg(y.getStepName(), "No ytt packages found"))
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
		if pkgValuesFile, err := y.app.prepareValuesFile(y.app.e.g.YttPkgStepDirName, pkgName); err != nil {
			log.Error().Err(err).Msg(y.app.Msg(y.getStepName(), "Unable to prepare a values file for the ytt package"))
			return "", err
		} else if pkgValuesFile != "" {
			yttArgs = []string{"--data-values-file=" + pkgValuesFile}
		}

		res, err := runYttWithFilesAndStdin(yttFiles, nil, func(name string, err error, stderr string, args []string) {
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
			return "", err
		}

		if res.Stdout == "" {
			log.Warn().Str("pkgName", pkgName).Msg(y.app.Msg(y.getStepName(), "No ytt package output"))
			continue
		}

		outputs = append(outputs, res.Stdout)
	}

	log.Info().Msg(y.app.Msg(y.getStepName(), "Ytt package rendered"))

	return strings.Join(outputs, "---\n"), nil
}

func (h *YttPkg) getStepName() string {
	return fmt.Sprintf("%s-%s", renderStepName, h.Ident())
}
