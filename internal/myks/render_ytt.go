package myks

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

func (a *Application) runYtt() (string, error) {
	yttDir, err := a.getVendoredResourceDir(a.e.g.YttStepDirName)
	if err != nil {
		log.Err(err).Str("app", a.Name).Msg("Unable to get helm charts dir")
		return "", err
	}
	yttDirs := getVendoredResourceDirs(yttDir)
	if len(yttDirs) == 0 {
		log.Debug().Str("app", a.Name).Msg("No resources to process")
		return "", nil
	}

	var outputs []string

	for _, resourceDir := range yttDirs {
		resourceName := filepath.Base(resourceDir)
		var resourceValuesFile string
		if resourceValuesFile, err = a.prepareValuesFile("ytt", resourceName); err != nil {
			log.Warn().Err(err).Str("app", a.Name).Msg("Unable to prepare helm values")
			return "", err
		}

		yttFiles, err := collectYttFiles(resourceDir)
		if err != nil {
			return "", err
		}

		var yttArgs []string
		if resourceValuesFile != "" {
			yttArgs = append(yttArgs, "--data-values-file="+resourceValuesFile)
		}

		res, err := runYttWithFilesAndStdin(yttFiles, nil, yttArgs...)
		if err != nil {
			log.Error().Err(err).Str("app", a.Name).Str("stdout", res.Stdout).Str("stderr", res.Stderr).Msg("Unable to run ytt")
			return "", err
		}

		if res.Stdout == "" {
			log.Warn().Str("app", a.Name).Str("resource", resourceName).Msg("No ytt output")
			continue
		}

		outputs = append(outputs, res.Stdout)
	}

	return strings.Join(outputs, "---\n"), nil
}

func collectYttFiles(resourceDir string) ([]string, error) {

	var yttFiles []string

	err := filepath.Walk(resourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && path != resourceDir {
			if !strings.HasPrefix(info.Name(), ".") {
				yttFiles = append(yttFiles, path)
			}
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return yttFiles, err
	}

	if len(yttFiles) == 0 {
		yttFiles = append(yttFiles, resourceDir)
	}

	return yttFiles, nil
}
