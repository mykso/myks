package myks

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/creasty/defaults"
	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

//go:embed assets/data-schema.ytt.yaml
var dataSchema []byte

//go:embed assets/envs_gitignore
var envsGitignore []byte

//go:embed all:assets/prototypes
var prototypesFs embed.FS

//go:embed all:assets/envs
var environmentsFs embed.FS

var GlobalLogFormat = "\033[1m[global]\033[0m %s"

// Define the main structure
type Globe struct {
	/// Globe configuration

	// Base directory for environments
	EnvironmentBaseDir string `default:"envs" yaml:"environmentBaseDir"`
	// Prefix for kubernetes namespaces
	NamespacePrefix string `default:"" yaml:"namespacePrefix"`
	// Application prototypes directory
	PrototypesDir string `default:"prototypes" yaml:"prototypesDir"`
	// Rendered kubernetes manifests directory
	RenderedDir string `default:"rendered" yaml:"renderedDir"`
	// Project root directory
	RootDir string `default:"." yaml:"rootDir"`

	/// Globe constants

	// Application data file name
	ApplicationDataFileName string `default:"app-data.ytt.yaml" yaml:"applicationDataFileName"`
	// Data values schema file name
	DataSchemaFileName string `default:"data-schema.ytt.yaml" yaml:"dataSchemaFileName"`
	// Environment data file name
	EnvironmentDataFileName string `default:"env-data.ytt.yaml" yaml:"environmentDataFileName"`
	// Helm charts directory name
	HelmChartsDirName string `default:"charts" yaml:"helmChartsDirName"`
	// Myks runtime config file name
	MyksDataFileName string `default:"myks-data.ytt.yaml" yaml:"myksDataFileName"`
	// Rendered environment data file name
	RenderedEnvironmentDataFileName string `default:"env-data.yaml" yaml:"renderedEnvironmentDataFileName"`
	// Service directory name
	ServiceDirName string `default:".myks" yaml:"serviceDirName"`
	// Temporary directory name
	TempDirName string `default:"tmp" yaml:"tempDirName"`
	// Rendered vendir config file name
	VendirConfigFileName string `default:"vendir.yaml" yaml:"vendirConfigFileName"`
	// Rendered vendir lock file name
	VendirLockFileName string `default:"vendir.lock.yaml" yaml:"vendirLockFileName"`
	// Rendered vendir sync file name
	VendirSyncFileName string `default:"vendir.sync.yaml" yaml:"vendirSyncFileName"`
	// Downloaded third-party sources
	VendorDirName string `default:"vendor" yaml:"vendorDirName"`
	// Ytt library directory name
	YttLibraryDirName string `default:"lib" yaml:"yttLibraryDirName"`
	// Ytt step directory name
	YttPkgStepDirName string `default:"ytt-pkg" yaml:"yttPkgStepDirName"`
	// Ytt step directory name
	YttStepDirName string `default:"ytt" yaml:"yttStepDirName"`

	/// User input

	// Application names to process
	ApplicationNames []string `yaml:"applicationNames"`
	// Paths to scan for environments
	SearchPaths []string `yaml:"searchPaths"`

	/// Runtime data

	// Git repository branch
	GitRepoBranch string `yaml:"gitRepoBranch"`
	// Git repository URL
	GitRepoUrl string `yaml:"gitRepoUrl"`

	// Collected environments for processing
	environments map[string]*Environment

	// Extra ytt file paths
	extraYttPaths []string
}

func New(rootDir string) *Globe {
	g := &Globe{
		RootDir:      rootDir,
		environments: make(map[string]*Environment),
	}
	if err := defaults.Set(g); err != nil {
		log.Fatal().Err(err).Msg("Unable to set defaults")
	}

	if err := g.setGitRepoUrl(); err != nil {
		log.Warn().Err(err).Msg("Unable to set git repo url")
	}

	if err := g.setGitRepoBranch(); err != nil {
		log.Warn().Err(err).Msg("Unable to set git repo branch")
	}

	log.Debug().Interface("globe", g).Msg("Globe config")
	return g
}

func (g *Globe) Init(searchPaths []string, applicationNames []string) error {
	g.SearchPaths = searchPaths
	g.ApplicationNames = applicationNames

	yttLibraryDir := filepath.Join(g.RootDir, g.YttLibraryDirName)
	if _, err := os.Stat(yttLibraryDir); err == nil {
		g.extraYttPaths = append(g.extraYttPaths, yttLibraryDir)
	}

	dataSchemaFileName := filepath.Join(g.RootDir, g.EnvironmentBaseDir, g.DataSchemaFileName)
	if _, err := os.Stat(dataSchemaFileName); err != nil {
		log.Warn().Err(err).Msg("Unable to find data schema file, creating one")
		if err := os.WriteFile(dataSchemaFileName, dataSchema, 0o600); err != nil {
			log.Fatal().Err(err).Msg("Unable to create data schema file")
		}
	}
	g.extraYttPaths = append(g.extraYttPaths, dataSchemaFileName)

	if configFileName, err := g.dumpConfigAsYaml(); err != nil {
		log.Warn().Err(err).Msg("Unable to dump config as yaml")
	} else {
		g.extraYttPaths = append(g.extraYttPaths, configFileName)
	}

	g.collectEnvironments(searchPaths)

	return processItemsInParallel(g.environments, func(item interface{}) error {
		env, ok := item.(*Environment)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Environment")
		}
		return env.Init(applicationNames)
	})
}

func (g *Globe) Sync() error {
	return processItemsInParallel(g.environments, func(item interface{}) error {
		env, ok := item.(*Environment)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Environment")
		}
		return env.Sync()
	})
}

func (g *Globe) Render() error {
	return processItemsInParallel(g.environments, func(item interface{}) error {
		env, ok := item.(*Environment)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Environment")
		}
		return env.Render()
	})
}

func (g *Globe) SyncAndRender() error {
	return processItemsInParallel(g.environments, func(item interface{}) error {
		env, ok := item.(*Environment)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Environment")
		}
		return env.SyncAndRender()
	})
}

// Bootstrap creates the initial directory structure and files
func (g *Globe) Bootstrap() error {
	log.Info().Msg("Creating base file structure")
	if err := g.createBaseFileStructure(); err != nil {
		return err
	}

	log.Info().Msg("Creating sample prototypes")
	if err := g.createSamplePrototypes(); err != nil {
		return err
	}

	log.Info().Msg("Creating sample environment")
	if err := g.createSampleEnvironment(); err != nil {
		return err
	}

	return nil
}

// dumpConfigAsYaml dumps the globe config as yaml to a file and returns the file name
func (g *Globe) dumpConfigAsYaml() (string, error) {
	configData := struct {
		Myks *Globe `yaml:"myks"`
	}{
		Myks: g,
	}
	var yamlData bytes.Buffer
	enc := yaml.NewEncoder(&yamlData)
	enc.SetIndent(2)
	if err := enc.Encode(configData); err != nil {
		return "", err
	}
	yttData := fmt.Sprintf("#@data/values\n---\n%s", yamlData.String())

	configFileName := filepath.Join(g.RootDir, g.ServiceDirName, g.TempDirName, g.MyksDataFileName)
	if err := os.MkdirAll(filepath.Dir(configFileName), 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(configFileName, []byte(yttData), 0o600); err != nil {
		return "", err
	}

	log.Trace().Str("config file", configFileName).Str("content", yttData).Msg("Dumped config as yaml")

	return configFileName, nil
}

func (g *Globe) createBaseFileStructure() error {
	envDir := filepath.Join(g.RootDir, g.EnvironmentBaseDir)
	protoDir := filepath.Join(g.RootDir, g.PrototypesDir)
	renderedDir := filepath.Join(g.RootDir, g.RenderedDir)
	dataSchemaFile := filepath.Join(envDir, g.DataSchemaFileName)
	envsGitignoreFile := filepath.Join(envDir, ".gitignore")

	log.Debug().Str("environments directory", envDir).Msg("")
	log.Debug().Str("prototypes directory", protoDir).Msg("")
	log.Debug().Str("rendered directory", renderedDir).Msg("")
	log.Debug().Str("data schema file", dataSchemaFile).Msg("")
	log.Debug().Str("environments .gitignore file", envsGitignoreFile).Msg("")

	// TODO: interactively ask for confirmation and overwrite without checking
	notCleanErr := fmt.Errorf("Target directory is not clean, aborting")

	if _, err := os.Stat(envDir); err == nil {
		return notCleanErr
	}
	if err := os.MkdirAll(envDir, 0o750); err != nil {
		return err
	}

	if _, err := os.Stat(protoDir); err == nil {
		return notCleanErr
	}
	if err := os.MkdirAll(protoDir, 0o750); err != nil {
		return err
	}

	if _, err := os.Stat(renderedDir); err == nil {
		return notCleanErr
	}
	if err := os.MkdirAll(renderedDir, 0o750); err != nil {
		return err
	}

	if _, err := os.Stat(dataSchemaFile); err == nil {
		return notCleanErr
	}
	if err := os.WriteFile(dataSchemaFile, dataSchema, 0o600); err != nil {
		return err
	}

	if _, err := os.Stat(envsGitignoreFile); err == nil {
		return notCleanErr
	}
	if err := os.WriteFile(envsGitignoreFile, envsGitignore, 0o600); err != nil {
		return err
	}

	return nil
}

func (g *Globe) createSamplePrototypes() error {
	protoDir := filepath.Join(g.RootDir, g.PrototypesDir)
	return copyFileSystemToPath(prototypesFs, "assets/prototypes", protoDir)
}

func (g *Globe) createSampleEnvironment() error {
	envDir := filepath.Join(g.RootDir, g.EnvironmentBaseDir)
	return copyFileSystemToPath(environmentsFs, "assets/envs", envDir)
}

func (g *Globe) collectEnvironments(searchPaths []string) {
	if len(searchPaths) == 0 {
		searchPaths = []string{g.EnvironmentBaseDir}
	}

	for _, searchPath := range searchPaths {
		g.collectEnvironmentsInPath(searchPath)
	}

	log.Debug().Interface("environments", g.environments).Msg(g.Msg("Collected environments"))
}

func (g *Globe) collectEnvironmentsInPath(searchPath string) {
	err := filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d != nil && d.IsDir() {
			_, err := os.Stat(filepath.Join(path, g.EnvironmentDataFileName))
			if err == nil {
				env := NewEnvironment(g, path)
				if env != nil {
					g.environments[path] = env
				} else {
					log.Warn().Str("path", path).Msg("Unable to collect environment, skipping")
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to walk environment directories")
	}
}

func (g *Globe) setGitRepoUrl() error {
	if g.GitRepoUrl == "" {
		result, err := runCmd("git", nil, []string{"remote", "get-url", "origin"}, func(name string, args []string) {
			log.Debug().Msg(msgRunCmd("set git repository url", name, args))
		})
		if err != nil {
			return err
		}
		g.GitRepoUrl = strings.Trim(result.Stdout, "\n")
	}
	return nil
}

func (g *Globe) setGitRepoBranch() error {
	if g.GitRepoBranch == "" {
		result, err := runCmd("git", nil, []string{"rev-parse", "--abbrev-ref", "HEAD"}, func(name string, args []string) {
			log.Debug().Msg(msgRunCmd("set git repository branch", name, args))
		})
		if err != nil {
			return err
		}
		g.GitRepoBranch = strings.Trim(result.Stdout, "\n")
	}
	return nil
}

func (g *Globe) Msg(msg string) string {
	formattedMessage := fmt.Sprintf(GlobalLogFormat, msg)
	return formattedMessage
}
