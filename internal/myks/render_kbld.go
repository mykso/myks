package myks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

const kbldOverrideFileName = "kbld-overrides.yaml"

type Kbld struct {
	ident    string
	app      *Application
	additive bool
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

	if len(config.Overrides) > 0 {
		overridesFilePath, err := k.generateOverridesConfig(previousStepFile, config)
		if err != nil {
			log.Warn().Err(err).Msg(k.app.Msg(k.getStepName(), "Unable to generate kbld overrides config"))
			return "", err
		}
		if overridesFilePath != "" {
			cmdArgs = append(cmdArgs, "--file="+overridesFilePath)
		}
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
	dataValuesYaml, err := k.app.ytt(k.getStepName(), "get kbld config", k.app.yttDataFiles, "--data-values-inspect")
	if err != nil {
		return KbldConfig{}, err
	}

	return newKbldConfig(dataValuesYaml.Stdout)
}

// generateOverridesConfig detects images using kbld --unresolved-inspect,
// and generates a kbld config file with image overrides based on the provided KbldConfig.
func (k *Kbld) generateOverridesConfig(inputFile string, config KbldConfig) (string, error) {
	cmdArgs := []string{
		"kbld",
		"--file=" + inputFile,
		"--unresolved-inspect",
	}

	cmdLogFn := func(name string, err error, stderr string, args []string) {
		purpose := k.getStepName() + " detect images"
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
		return "", fmt.Errorf("failed to detect images: %w", err)
	}

	if res.Stdout == "" {
		log.Debug().Msg(k.app.Msg(k.getStepName(), "No images detected by kbld"))
		return inputFile, nil
	}

	prefix := "- image: "
	lines := strings.Split(strings.TrimSpace(res.Stdout), "\n")
	var imageRefs []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, prefix); ok {
			imageRefs = append(imageRefs, after)
		}
	}

	if len(imageRefs) == 0 {
		log.Debug().Msg(k.app.Msg(k.getStepName(), "No valid image references found"))
		return "", nil
	}

	overrideMap := make(map[string]string)
	for _, imageRef := range imageRefs {
		newImageRef, err := config.applyOverrides(imageRef)
		if err != nil {
			log.Warn().Err(err).Str("image", imageRef).Msg(k.app.Msg(k.getStepName(), "Failed to apply overrides"))
			continue
		}
		if newImageRef != "" {
			overrideMap[imageRef] = newImageRef
			log.Debug().Str("from", imageRef).Str("to", newImageRef).Msg(k.app.Msg(k.getStepName(), "Image override applied"))
		}
	}

	if len(overrideMap) == 0 {
		log.Debug().Msg(k.app.Msg(k.getStepName(), "No overrides applied"))
		return "", nil
	}

	return k.generateKbldOverrideConfig(overrideMap)
}

// generateKbldOverrideConfig creates a kbld config file with image overrides
func (k *Kbld) generateKbldOverrideConfig(overrides map[string]string) (string, error) {
	type kbldOverride struct {
		Image    string `yaml:"image"`
		NewImage string `yaml:"newImage"`
	}

	type kbldConfig struct {
		APIVersion string         `yaml:"apiVersion"`
		Kind       string         `yaml:"kind"`
		Overrides  []kbldOverride `yaml:"overrides"`
	}

	config := kbldConfig{
		APIVersion: "kbld.k14s.io/v1alpha1",
		Kind:       "Config",
	}

	for oldImage, newImage := range overrides {
		config.Overrides = append(config.Overrides, kbldOverride{
			Image:    oldImage,
			NewImage: newImage,
		})
	}

	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal kbld config: %w", err)
	}

	err = k.app.writeServiceFile(kbldOverrideFileName, string(yamlBytes))
	filename := k.app.expandServicePath(kbldOverrideFileName)
	return filename, err
}

func (k *Kbld) getStepName() string {
	return fmt.Sprintf("%s-%s", renderStepName, k.Ident())
}
