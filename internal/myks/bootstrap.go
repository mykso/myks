package myks

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

//go:embed assets/data-schema.ytt.yaml
var dataSchema []byte

//go:embed assets/gitignore
var gitignore []byte

//go:embed assets/myks_config.yaml
var myksConfig []byte

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
func (g *Globe) Bootstrap(force, onlyPrint bool, components []string) error {
	compMap := make(map[string]bool, len(components))
	for _, comp := range components {
		compMap[comp] = true
	}

	if onlyPrint {
		if compMap["gitignore"] {
			printFileNicely(".gitignore", string(gitignore), "Terminfo")
		}
		if compMap["config"] {
			printFileNicely(".myks.yaml", string(myksConfig), "YAML")
		}
		if compMap["schema"] {
			printFileNicely("data-schema.ytt.yaml", string(dataSchema), "YAML")
		}
	} else {
		log.Info().Msg("Creating base file structure")
		if err := g.createBaseFileStructure(force); err != nil {
			return err
		}
	}

	if compMap["prototypes"] {
		if onlyPrint {
			log.Info().Msg("Skipping printing sample prototypes")
		} else {
			log.Info().Msg("Creating sample prototypes")
			if err := g.createSamplePrototypes(); err != nil {
				return err
			}
		}
	}

	if compMap["environments"] {
		if onlyPrint {
			log.Debug().Msg("Skipping printing sample environment")
		} else {
			log.Info().Msg("Creating sample environment")
			if err := g.createSampleEnvironment(); err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *Globe) createBaseFileStructure(force bool) error {
	envDir := filepath.Join(g.RootDir, g.EnvironmentBaseDir)
	protoDir := filepath.Join(g.RootDir, g.PrototypesDir)
	renderedDir := filepath.Join(g.RootDir, g.RenderedEnvsDir)
	gitignoreFile := filepath.Join(g.RootDir, ".gitignore")
	myksConfigFile := filepath.Join(g.RootDir, ".myks.yaml")

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

	g.createDataSchemaFile()

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

	if err := os.WriteFile(myksConfigFile, myksConfig, 0o600); err != nil {
		return err
	}

	return nil
}

func (g *Globe) createDataSchemaFile() string {
	dataSchemaFileName := filepath.Join(g.RootDir, g.ServiceDirName, g.TempDirName, g.DataSchemaFileName)
	if ok, err := isExist(dataSchemaFileName); err != nil {
		log.Fatal().Err(err).Msg("Unable to stat data schema file")
	} else if !ok {
		log.Debug().Msg("Unable to find data schema file, creating one")
		if err := os.MkdirAll(filepath.Dir(dataSchemaFileName), 0o750); err != nil {
			log.Fatal().Err(err).Msg("Unable to create data schema file directory")
		}
	} else {
		log.Debug().Msg("Overwriting existing data schema file")
	}
	if err := os.WriteFile(dataSchemaFileName, dataSchema, 0o600); err != nil {
		log.Fatal().Err(err).Msg("Unable to create data schema file")
	}
	return dataSchemaFileName
}

func (g *Globe) createSamplePrototypes() error {
	protoDir := filepath.Join(g.RootDir, g.PrototypesDir)
	return copyFileSystemToPath(prototypesFs, "assets/prototypes", protoDir)
}

func (g *Globe) createSampleEnvironment() error {
	envDir := filepath.Join(g.RootDir, g.EnvironmentBaseDir)
	return copyFileSystemToPath(environmentsFs, "assets/envs", envDir)
}
