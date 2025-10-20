package myks

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

type Kbld struct {
	ident    string
	app      *Application
	additive bool
}

type KbldConfig struct {
	Enabled          bool `yaml:"enabled"`
	ImagesAnnotation bool `yaml:"imagesAnnotation"`
	Cache            bool `yaml:"cache"`
}

func (k *Kbld) IsAdditive() bool {
	return k.additive
}

func (k *Kbld) Ident() string {
	return k.ident
}

func (k *Kbld) Render(previousStepFile string) (string, error) {
	config, err := k.getKbldConfig()
	if err != nil {
		log.Warn().Err(err).Msg(k.app.Msg(k.getStepName(), "Unable to get kbld config"))
		return "", err
	}

	if !config.Enabled {
		log.Debug().Msg(k.app.Msg(k.getStepName(), "Kbld is disabled in configuration, skipping"))
		// just read the previous step file and return its content
		// TODO: implement skipping for "rendering tools" properly
		data, err := os.ReadFile(filepath.Clean(previousStepFile))
		if err != nil {
			log.Warn().Err(err).Str("file", previousStepFile).Msg(k.app.Msg(k.getStepName(), "Unable to read previous step file"))
			return "", err
		}
		return string(data), nil
	}

	lockFileName := "kbld-lock.yaml"
	lockFilePath := k.app.expandServicePath(lockFileName)

	cmdArgs := []string{
		"kbld",
		"--file=" + previousStepFile,
		// Use --imgpkg-lock-output instead of --lock-output due to a kbld bug.
		// If kbld is embedded, its version is set after the dependency version, which has the `v` prefix.
		// The version is always written into the lock file, and on subsequent runs kbld fails to validate the lock
		// file, because the `v` prefix is not allowed in the minimumRequiredVersion field.
		"--imgpkg-lock-output=" + lockFilePath,
		fmt.Sprintf("--images-annotation=%t", config.ImagesAnnotation),
	}

	// if cache is enabled, check existence of the lock file and include it in the args
	if config.Cache {
		if ok, err := isExist(lockFilePath); ok {
			log.Debug().Str("file", lockFilePath).Msg(k.app.Msg(k.getStepName(), "Using existing kbld lock file for caching"))
			cmdArgs = append(cmdArgs, "--file="+lockFilePath)
		} else if err == nil {
			log.Debug().Str("file", lockFilePath).Msg(k.app.Msg(k.getStepName(), "Kbld lock file not found, proceeding without cache"))
		} else {
			log.Warn().Err(err).Str("file", lockFilePath).Msg(k.app.Msg(k.getStepName(), "Error checking kbld lock file existence"))
		}
	}

	cmdLogFn := func(name string, err error, stderr string, args []string) {
		purpose := k.getStepName() + " render step"
		cmd := msgRunCmd(purpose, name, args)
		if err != nil {
			log.Error().Msg(cmd)
			log.Error().Msg(stderr)
		} else {
			log.Debug().Msg(cmd)
		}
	}
	res, err := runCmd(myksFullPath(), nil, cmdArgs, cmdLogFn)
	if err != nil {
		return "", err
	}

	if res.Stdout == "" {
		log.Warn().Msg(k.app.Msg(k.getStepName(), "Empty kbld output"))
		return "", nil
	}

	log.Info().Msg(k.app.Msg(k.getStepName(), "kbld rendered"))

	return res.Stdout, nil
}

func (k *Kbld) getKbldConfig() (KbldConfig, error) {
	var kbldConfigWrapper struct {
		Kbld KbldConfig `yaml:"kbld"`
	}

	dataValuesYaml, err := k.app.ytt(k.getStepName(), "get kbld config", k.app.yttDataFiles, "--data-values-inspect")
	if err != nil {
		return KbldConfig{}, err
	}

	if err := yaml.Unmarshal([]byte(dataValuesYaml.Stdout), &kbldConfigWrapper); err != nil {
		return KbldConfig{}, err
	}

	return KbldConfig{
		Enabled:          kbldConfigWrapper.Kbld.Enabled,
		ImagesAnnotation: kbldConfigWrapper.Kbld.ImagesAnnotation,
		Cache:            kbldConfigWrapper.Kbld.Cache,
	}, nil
}

func (k *Kbld) getStepName() string {
	return fmt.Sprintf("%s-%s", renderStepName, k.Ident())
}
