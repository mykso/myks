package kwhoosh

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

type Environment struct {
	// Path to the environment directory
	Dir string
	// Environment data file
	EnvironmentDataFile string
	// Environment id
	Id string
	// Kwhoosh instance
	k *Kwhoosh
}

func NewEnvironment(k *Kwhoosh, dir string) *Environment {
	envDataFile := filepath.Join(dir, k.EnvironmentDataFileName)

	env := &Environment{
		Dir:                 dir,
		EnvironmentDataFile: envDataFile,
		k:                   k,
	}

	if err := env.setId(); err != nil {
		log.Warn().Err(err).Str("dir", dir).Msg("Unable to set environment id")
		return nil
	}

	if err := env.renderManifest(); err != nil {
		log.Warn().Err(err).Str("dir", dir).Msg("Unable to render environment manifest")
		return nil
	}

	return env
}

func (e *Environment) setId() error {
	yamlBytes, err := os.ReadFile(e.EnvironmentDataFile)
	if err != nil {
		log.Debug().Err(err).Msg("Unable to read environment data file")
		return err
	}

	var envData struct {
		Environment struct {
			Id string
		}
	}
	err = yaml.Unmarshal(yamlBytes, &envData)
	if err != nil {
		log.Debug().Err(err).Msg("Unable to unmarshal environment data file")
		return err
	}

	log.Debug().Interface("envData", envData).Msg("Environment data")

	if envData.Environment.Id == "" {
		err = errors.New("Environment data file missing id")
		log.Debug().Err(err).Str("file", e.EnvironmentDataFile).Msg("Unable to set environment id")
		return err
	}

	e.Id = envData.Environment.Id

	return nil
}

// Get all environment manifest files up to the root directory
func (e *Environment) getManifestFiles() []string {
	currentPath := e.k.RootDir
	manifestFiles := []string{}
	for _, level := range strings.Split(e.Dir, filepath.FromSlash("/")) {
		currentPath = filepath.Join(currentPath, level)
		manifestFile := filepath.Join(currentPath, e.k.EnvironmentManifestFileName)
		if _, err := os.Stat(manifestFile); err == nil {
			manifestFiles = append(manifestFiles, manifestFile)
		}
	}
	log.Debug().Interface("manifestFiles", manifestFiles).Msg("Manifest files")
	return manifestFiles
}

// Render the final manifest file for the environment
func (e *Environment) renderManifest() error {
	manifestFiles := e.getManifestFiles()
	if len(manifestFiles) == 0 {
		return errors.New("No manifest files found")
	}
	res, err := YttFiles(manifestFiles)
	if err != nil {
		log.Error().Err(err).Str("stderr", res.Stderr).Msg("Unable to render environment manifest")
		return err
	}
	if res.Stdout == "" {
		return errors.New("Empty output from ytt")
	}
	renderedManifestFile := filepath.Join(e.Dir, e.k.RenderedEnvironmentManifestFileName)
	err = os.WriteFile(renderedManifestFile, []byte(res.Stdout), 0o644)
	if err != nil {
		log.Error().Err(err).Msg("Unable to write rendered manifest file")
		return err
	}
	log.Debug().Str("file", renderedManifestFile).Msg("Wrote rendered manifest file")
	return nil
}
