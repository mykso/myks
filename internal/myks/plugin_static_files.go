package myks

import (
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// StaticFilesStepName is the step name identifier for the static files copy plugin.
const StaticFilesStepName = "static-files"

// staticFilesSourceDirs returns the source directories containing static files for this application.
// It searches in:
//  1. prototypes/<prototype>/static/
//  2. envs/**/_proto/<prototype>/static/ (at each env hierarchy level)
//  3. envs/**/_env/static/ (at each env hierarchy level)
//  4. envs/**/_apps/<app>/static/ (at each env hierarchy level)
//
// Used by both static files copy and inspect.
func (a *Application) staticFilesSourceDirs() ([]string, error) {
	var dirs []string

	prototypeStaticFilesDir := filepath.Join(a.Prototype, a.cfg.StaticFilesDirName)
	if ok, err := isExist(prototypeStaticFilesDir); err != nil {
		return nil, err
	} else if ok {
		dirs = append(dirs, prototypeStaticFilesDir)
	}
	dirs = append(dirs, a.e.collectBySubpath(filepath.Join(a.cfg.PrototypeOverrideDir, a.prototypeDirName(), a.cfg.StaticFilesDirName))...)
	dirs = append(dirs, a.e.collectBySubpath(filepath.Join(a.cfg.EnvsDir, a.cfg.StaticFilesDirName))...)
	dirs = append(dirs, a.e.collectBySubpath(filepath.Join(a.cfg.AppsDir, a.Name, a.cfg.StaticFilesDirName))...)

	return dirs, nil
}

func (a *Application) copyStaticFiles() error {
	logStaticFiles := func(files []string) {
		log.Trace().Strs("staticFiles", files).Msg(a.Msg(StaticFilesStepName, "Static files"))
	}

	staticFilesDirs, err := a.staticFilesSourceDirs()
	if err != nil {
		return err
	}
	logStaticFiles(staticFilesDirs)

	staticFilesDestination := filepath.Join(a.getDestinationDir(), a.cfg.StaticFilesDirName)

	for _, staticFilesDir := range staticFilesDirs {
		if err := copyDir(staticFilesDir, staticFilesDestination, true); err != nil {
			log.Error().Err(err).Msg(a.Msg(StaticFilesStepName, "Unable to copy static files"))
			return err
		}
	}

	return nil
}
