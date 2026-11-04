package myks

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	gv "github.com/hashicorp/go-version"
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

// argocdFs mirrors assets/envs and assets/prototypes; its files are copied over theirs.
//
//go:embed all:assets/argocd
var argocdFs embed.FS

//go:embed assets/ci/github
var githubWorkflowsFs embed.FS

// ErrBootstrapTargetExists is returned when a bootstrap target path already exists.
type ErrBootstrapTargetExists struct {
	target string
}

func (e ErrBootstrapTargetExists) Error() string {
	return fmt.Sprintf("target '%s' already exists", e.target)
}

// Bootstrap creates the initial directory structure and files
func (g *Globe) Bootstrap(force, onlyPrint bool, components []string, version string) error {
	compMap := make(map[string]bool, len(components))
	for _, comp := range components {
		compMap[comp] = true
	}

	myksConfig := fmt.Sprintf(string(myksConfigTpl), version, g.generateNameConventions())

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
		if err := g.createBaseFileStructure(force, myksConfig); err != nil {
			return fmt.Errorf("creating base file structure: %w", err)
		}
	}

	if compMap["prototypes"] {
		if onlyPrint {
			log.Info().Msg("Skipping printing sample prototypes")
		} else {
			log.Info().Msg("Creating sample prototypes")
			if err := g.createSamplePrototypes(compMap["argocd"]); err != nil {
				return fmt.Errorf("creating sample prototypes: %w", err)
			}
		}
	}

	if compMap["environments"] {
		if onlyPrint {
			log.Debug().Msg("Skipping printing sample environment")
		} else {
			log.Info().Msg("Creating sample environment")
			if err := g.createSampleEnvironment(compMap["argocd"]); err != nil {
				return fmt.Errorf("creating sample environment: %w", err)
			}
		}
	}

	if compMap["ci-github"] {
		workflows, err := githubWorkflows(version)
		if err != nil {
			return err
		}
		for _, wf := range workflows {
			if onlyPrint {
				printFileNicely(wf.path, wf.content, "YAML")
			} else if err := g.writeNewFile(force, wf.path, wf.content); err != nil {
				return fmt.Errorf("creating GitHub workflows: %w", err)
			}
		}
	}

	return nil
}

type bootstrapFile struct {
	path    string
	content string
}

// githubWorkflows returns the CI workflows of docs/gitops-ci.md, pinned to the myks release that writes them.
func githubWorkflows(version string) ([]bootstrapFile, error) {
	tag := "latest"
	if v, err := gv.NewSemver(version); err == nil && v.Prerelease() == "" {
		tag = "v" + v.String()
	}
	entries, err := githubWorkflowsFs.ReadDir("assets/ci/github")
	if err != nil {
		return nil, err
	}
	files := make([]bootstrapFile, 0, len(entries))
	for _, e := range entries {
		content, err := githubWorkflowsFs.ReadFile("assets/ci/github/" + e.Name())
		if err != nil {
			return nil, err
		}
		files = append(files, bootstrapFile{
			path:    filepath.Join(".github", "workflows", e.Name()),
			content: strings.ReplaceAll(string(content), "__MYKS_VERSION__", tag),
		})
	}
	return files, nil
}

func (g *Globe) writeNewFile(force bool, path, content string) error {
	path = filepath.Join(g.RootDir, path)
	if !force {
		if ok, err := isExist(path); err != nil {
			return err
		} else if ok {
			return ErrBootstrapTargetExists{target: path}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func (g *Globe) createBaseFileStructure(force bool, myksConfig string) error {
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
		return fmt.Errorf("creating environments directory: %w", err)
	}

	if err := os.MkdirAll(protoDir, 0o750); err != nil {
		return fmt.Errorf("creating prototypes directory: %w", err)
	}

	if err := os.MkdirAll(renderedDir, 0o750); err != nil {
		return fmt.Errorf("creating rendered directory: %w", err)
	}

	if err := os.WriteFile(gitignoreFile, gitignore, 0o600); err != nil {
		return fmt.Errorf("writing .gitignore: %w", err)
	}

	if err := os.WriteFile(myksConfigFile, []byte(myksConfig), 0o600); err != nil {
		return fmt.Errorf("writing .myks.yaml config: %w", err)
	}

	return nil
}

func (g *Globe) createDataSchemaFile() string {
	dataSchemaFilePath := filepath.Join(g.RootDir, g.ServiceDirName, g.TempDirName, g.DataSchemaFileName)
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

func (g *Globe) createSamplePrototypes(argocd bool) error {
	protoDir := filepath.Join(g.RootDir, g.PrototypesDir)
	if err := copyFileSystemToPath(prototypesFs, "assets/prototypes", protoDir); err != nil || !argocd {
		return err
	}
	return copyFileSystemToPath(argocdFs, "assets/argocd/prototypes", protoDir)
}

func (g *Globe) createSampleEnvironment(argocd bool) error {
	envDir := filepath.Join(g.RootDir, g.EnvironmentBaseDir)
	if err := copyFileSystemToPath(environmentsFs, "assets/envs", envDir); err != nil || !argocd {
		return err
	}
	return copyFileSystemToPath(argocdFs, "assets/argocd/envs", envDir)
}

// generateNameConventions extracts mapstructure and default tags from Config struct
// and generates a YAML section for naming-conventions
func (g *Globe) generateNameConventions() string {
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
