package myks

import (
	"path/filepath"

	"github.com/rs/zerolog/log"
)

const StaticFilesStepName = "static-files"

func (a *Application) copyStaticFiles() error {
	if staticFilesDirs, err := a.collectTreeFiles(a.e.g.StaticFilesDirName); err == nil {
		staticFilesDestination := filepath.Join(a.getDestinationDir(), a.e.g.StaticFilesDirName)

		for _, staticFilesDir := range staticFilesDirs {
			if err = copyDir(staticFilesDir, staticFilesDestination, true); err != nil {
				log.Error().Err(err).Msg(a.Msg(StaticFilesStepName, "Unable to copy static files"))
				return err
			}
		}
		return nil
	} else {
		return err
	}
}
