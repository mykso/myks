package kwhoosh

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type Environment struct {
	// Path to the environment directory
	Dir string
	// Environment data file
	EnvironmentDataFile string
	// Environment id
	Id string
}

func NewEnvironment(k *Kwhoosh, dir string) *Environment {
	envDataFile := filepath.Join(dir, k.EnvironmentDataFileName)

	env := &Environment{
		Dir:                 dir,
		EnvironmentDataFile: envDataFile,
	}

	if err := env.setId(); err == nil {
		return env
	} else {
		log.Warn().Err(err).Str("dir", dir).Msg("Unable to set environment id")
		return nil
	}
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
