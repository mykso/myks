package myks

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/rs/zerolog/log"
)

//go:embed assets/data-schema.ytt.yaml
var dataSchema []byte

//go:embed assets/gitignore
var gitignore []byte

//go:embed assets/myks_config.tpl.yaml
var myksConfigTpl []byte

//go:embed all:assets/prototypes
var prototypesFs embed.FS

//go:embed all:assets/envs
var environmentsFs embed.FS

type ErrBootstrapTargetExists struct {
	target string
}

func (e ErrBootstrapTargetExists) Error() string {
	return fmt.Sprintf("target '%s' already exists", e.target)
}

// Bootstrap creates the initial directory structure and files
func Bootstrap(cfg Config, force, onlyPrint bool, components []string, version string) error {
	compMap := make(map[string]bool, len(components))
	for _, comp := range components {
		compMap[comp] = true
	}

	myksConfig := fmt.Sprintf(string(myksConfigTpl), version, generateNameConventions(cfg))

	if onlyPrint {
		if compMap["gitignore"] {
			printFileNicely(".gitignore", string(gitignore), "Terminfo")
		}
		if compMap["config"] {
			printFileNicely(".myks.yaml", myksConfig, "YAML")
		}
		if compMap["schema"] {
			printFileNicely("data-schema.ytt.yaml", string(dataSchema), "YAML")
		}
	} else {
		log.Info().Msg("Creating base file structure")
		if err := createBaseFileStructure(cfg, force, myksConfig); err != nil {
			return err
		}
	}

	if compMap["prototypes"] {
		if onlyPrint {
			log.Info().Msg("Skipping printing sample prototypes")
		} else {
			log.Info().Msg("Creating sample prototypes")
			if err := createSamplePrototypes(cfg); err != nil {
				return err
			}
		}
	}

	if compMap["environments"] {
		if onlyPrint {
			log.Debug().Msg("Skipping printing sample environment")
		} else {
			log.Info().Msg("Creating sample environment")
			if err := createSampleEnvironment(cfg); err != nil {
				return err
			}
		}
	}

	return nil
}

func createBaseFileStructure(cfg Config, force bool, myksConfig string) error {
	envDir := filepath.Join(cfg.RootDir, cfg.EnvironmentBaseDir)
	protoDir := filepath.Join(cfg.RootDir, cfg.PrototypesDir)
	renderedDir := filepath.Join(cfg.RootDir, cfg.RenderedEnvsDir)
	gitignoreFile := filepath.Join(cfg.RootDir, ".gitignore")
	myksConfigFile := filepath.Join(cfg.RootDir, ".myks.yaml")

	log.Debug().Str("environments directory", envDir).Msg("")
	log.Debug().Str("prototypes directory", protoDir).Msg("")
	log.Debug().Str("rendered directory", renderedDir).Msg("")
	log.Debug().Str(".gitignore file", gitignoreFile).Msg("")
	log.Debug().Str("myks config file", myksConfigFile).Msg("")

	if !force {
		for _, path := range []string{envDir, protoDir, renderedDir, gitignoreFile, myksConfigFile} {
			ok, err := isExist(path)
			if err != nil {
				return err
			}
			if ok {
				return ErrBootstrapTargetExists{target: path}
			}
		}
	}

	createDataSchemaFile(cfg)

	if err := os.MkdirAll(envDir, 0o750); err != nil {
		return err
	}

	if err := os.MkdirAll(protoDir, 0o750); err != nil {
		return err
	}

	if err := os.MkdirAll(renderedDir, 0o750); err != nil {
		return err
	}

	if err := os.WriteFile(gitignoreFile, gitignore, 0o600); err != nil {
		return err
	}

	if err := os.WriteFile(myksConfigFile, []byte(myksConfig), 0o600); err != nil {
		return err
	}

	return nil
}

func createDataSchemaFile(cfg Config) string {
	dataSchemaFilePath := filepath.Join(cfg.RootDir, cfg.ServiceDirName, cfg.TempDirName, cfg.DataSchemaFileName)
	log.Debug().Str("dataSchemaFilePath", dataSchemaFilePath).Msg("Ensuring data schema file exists")
	if ok, err := isExist(dataSchemaFilePath); err != nil {
		log.Fatal().Err(err).Msg("Unable to stat data schema file")
	} else if !ok {
		if err := os.MkdirAll(filepath.Dir(dataSchemaFilePath), 0o750); err != nil {
			log.Fatal().Err(err).Msg("Unable to create data schema file directory")
		}
	} else {
		log.Debug().Msg("Overwriting existing data schema file")
	}
	if err := os.WriteFile(dataSchemaFilePath, dataSchema, 0o600); err != nil {
		log.Fatal().Err(err).Msg("Unable to create data schema file")
	}
	return dataSchemaFilePath
}

func createSamplePrototypes(cfg Config) error {
	protoDir := filepath.Join(cfg.RootDir, cfg.PrototypesDir)
	return copyFileSystemToPath(prototypesFs, "assets/prototypes", protoDir)
}

func createSampleEnvironment(cfg Config) error {
	envDir := filepath.Join(cfg.RootDir, cfg.EnvironmentBaseDir)
	return copyFileSystemToPath(environmentsFs, "assets/envs", envDir)
}

// generateNameConventions extracts mapstructure and default tags from Config struct
// and generates a YAML section for naming-conventions
func generateNameConventions(cfg Config) string {
	var conventions strings.Builder

	rt := reflect.TypeFor[Config]()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)

		mapstructureTag := field.Tag.Get("mapstructure")
		if mapstructureTag == "" {
			continue
		}

		defaultTag := field.Tag.Get("default")
		if defaultTag == "" {
			continue
		}

		fmt.Fprintf(&conventions, "  %s: %s\n", mapstructureTag, defaultTag)
	}

	return conventions.String()
}
