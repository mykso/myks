package myks

import (
	"path/filepath"

	"github.com/rs/zerolog/log"
)

const StaticFilesStepName = "static-files"

func (a *Application) copyStaticFiles() (err error) {
	logStaticFiles := func(files []string) {
		log.Trace().Strs("staticFiles", files).Msg(a.Msg(StaticFilesStepName, "Static files"))
	}
	staticFilesDirs := []string{}

	// 1. Static files from the prototype
	prototypeStaticFilesDir := filepath.Join(a.Prototype, a.e.g.StaticFilesDirName)
	if ok, err := isExist(prototypeStaticFilesDir); err != nil {
		return err
	} else if ok {
		staticFilesDirs = append(staticFilesDirs, prototypeStaticFilesDir)
	}
	logStaticFiles(staticFilesDirs)
	// 2. Static files from prototype overrides
	staticFilesDirs = append(staticFilesDirs, a.e.collectBySubpath(filepath.Join("_proto", a.prototypeDirName(), a.e.g.StaticFilesDirName))...)
	logStaticFiles(staticFilesDirs)

	// 3. Static files from the environment
	staticFilesDirs = append(staticFilesDirs, a.e.collectBySubpath(filepath.Join(a.e.g.EnvsDir, a.e.g.StaticFilesDirName))...)
	logStaticFiles(staticFilesDirs)
	// 4. Static files from the application
	staticFilesDirs = append(staticFilesDirs, a.e.collectBySubpath(filepath.Join("_apps", a.Name, a.e.g.StaticFilesDirName))...)
	logStaticFiles(staticFilesDirs)

	staticFilesDestination := filepath.Join(a.getDestinationDir(), a.e.g.StaticFilesDirName)

	for _, staticFilesDir := range staticFilesDirs {
		if err = copyDir(staticFilesDir, staticFilesDestination, true); err != nil {
			log.Error().Err(err).Msg(a.Msg(StaticFilesStepName, "Unable to copy static files"))
			return err
		}
	}

	return nil
}
