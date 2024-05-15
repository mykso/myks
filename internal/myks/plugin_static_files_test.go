package myks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/creasty/defaults"
	"github.com/rs/zerolog/log"
)

func TestApplication_copyStaticFiles(t *testing.T) {
	defer chdir(t, "../../testData/static-test")()

	envName := "static-test-env"
	appName := "static-test-app"
	protoName := "static-test-proto"

	globe := &Globe{}
	if err := defaults.Set(globe); err != nil {
		log.Fatal().Err(err).Msg("Unable to set defaults")
	}

	env := &Environment{
		ID:  envName,
		g:   globe,
		Dir: filepath.Join(globe.EnvironmentBaseDir, envName),
	}

	app := &Application{
		Name:      appName,
		Prototype: fmt.Sprintf("%s/%s", globe.PrototypesDir, protoName),
		e:         env,
	}

	targetDir := app.getDestinationDir()

	isIgnored := func(info os.DirEntry) bool {
		return info.IsDir() || info.Name() == ".keep" || info.Name() == ".gitignore"
	}

	// Cleanup copied files
	defer func() {
		if err := filepath.WalkDir(targetDir, func(path string, info os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if isIgnored(info) {
				return nil
			}
			return os.Remove(path)
		}); err != nil {
			t.Errorf("copyStaticFiles() error = %v", err)
		}
	}()

	if err := app.copyStaticFiles(); err != nil {
		t.Errorf("copyStaticFiles() error = %v", err)
	}

	expectedFiles := map[string]string{
		"/static/app.txt":            "",
		"/static/app_override.txt":   "",
		"/static/conflict.txt":       "application",
		"/static/env.txt":            "",
		"/static/env_override.txt":   "",
		"/static/proto.txt":          "",
		"/static/proto_override.txt": "",
	}

	generatedFiles := map[string]string{}
	if err := filepath.WalkDir(targetDir, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if isIgnored(info) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		path = strings.TrimPrefix(path, targetDir)
		generatedFiles[path] = strings.Trim(string(content), "\n")

		return nil
	}); err != nil {
		t.Errorf("copyStaticFiles() error = %v", err)
	}

	assertEqual(t, generatedFiles, expectedFiles)
}
