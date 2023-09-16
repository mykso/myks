package myks

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/creasty/defaults"
	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

const GlobalLogFormat = "\033[1m[global]\033[0m %s"

var ErrNotClean = fmt.Errorf("target directory is not clean, aborting")

// Define the main structure
type Globe struct {
	/// Globe configuration

	// Base directory for environments
	EnvironmentBaseDir string `default:"envs"`
	// Main branch name
	MainBranchName string `default:"main"`
	// Prefix for kubernetes namespaces
	NamespacePrefix string `default:""`
	// Application prototypes directory
	PrototypesDir string `default:"prototypes"`
	// Rendered kubernetes manifests directory
	RenderedDir string `default:"rendered"`
	// Project root directory
	RootDir string `default:"."`

	/// Globe constants

	// Application data file name
	ApplicationDataFileName string `default:"app-data.ytt.yaml"`
	// ArgoCD data directory name
	ArgoCDDataDirName string `default:"argocd"`
	// Data values schema file name
	DataSchemaFileName string `default:"data-schema.ytt.yaml"`
	// Environment data file name
	EnvironmentDataFileName string `default:"env-data.ytt.yaml"`
	// Helm charts directory name
	HelmChartsDirName string `default:"charts"`
	// Myks runtime data file name
	MyksDataFileName string `default:"myks-data.ytt.yaml"`
	// Rendered environment data file name
	RenderedEnvironmentDataFileName string `default:"env-data.yaml"`
	// Service directory name
	ServiceDirName string `default:".myks"`
	// Temporary directory name
	TempDirName string `default:"tmp"`
	// Rendered vendir config file name
	VendirConfigFileName string `default:"vendir.yaml"`
	// Rendered vendir lock file name
	VendirLockFileName string `default:"vendir.lock.yaml"`
	// Rendered vendir sync file name
	VendirSyncFileName string `default:"vendir.sync.yaml"`
	// Prefix for vendir secret environment variables
	VendirSecretEnvPrefix string `default:"VENDIR_SECRET_"`
	// Downloaded third-party sources
	VendorDirName string `default:"vendor"`
	// Ytt library directory name
	YttLibraryDirName string `default:"lib"`
	// Ytt step directory name
	YttPkgStepDirName string `default:"ytt-pkg"`
	// Ytt step directory name
	YttStepDirName string `default:"ytt"`

	/// User input

	// Application names to process
	ApplicationNames []string
	// Paths to scan for environments
	SearchPaths []string

	/// Runtime data

	// Git repository path prefix (non-empty if running in a subdirectory of a git repository)
	GitPathPrefix string
	// Git repository branch
	GitRepoBranch string
	// Git repository URL
	GitRepoUrl string

	// Collected environments for processing
	environments map[string]*Environment

	// Extra ytt file paths
	extraYttPaths []string
}

// YttGlobe controls runtime data available to ytt templates
type YttGlobeData struct {
	GitRepoBranch string `yaml:"gitRepoBranch"`
	GitRepoUrl    string `yaml:"gitRepoUrl"`
}

type VendirCredentials struct {
	Username string
	Password string
}

func New(rootDir string) *Globe {
	g := &Globe{
		RootDir:      rootDir,
		environments: make(map[string]*Environment),
	}
	if err := defaults.Set(g); err != nil {
		log.Fatal().Err(err).Msg("Unable to set defaults")
	}

	if err := g.setGitPathPrefix(); err != nil {
		log.Warn().Err(err).Msg("Unable to set git path prefix")
	}

	if err := g.setGitRepoUrl(); err != nil {
		log.Warn().Err(err).Msg("Unable to set git repo url")
	}

	if err := g.setGitRepoBranch(); err != nil {
		log.Warn().Err(err).Msg("Unable to set git repo branch")
	}

	yttLibraryDir := filepath.Join(g.RootDir, g.YttLibraryDirName)
	if _, err := os.Stat(yttLibraryDir); err == nil {
		g.extraYttPaths = append(g.extraYttPaths, yttLibraryDir)
	}

	g.extraYttPaths = append(g.extraYttPaths, g.createDataSchemaFile())

	if configFileName, err := g.dumpConfigAsYaml(); err != nil {
		log.Warn().Err(err).Msg("Unable to dump config as yaml")
	} else {
		g.extraYttPaths = append(g.extraYttPaths, configFileName)
	}

	log.Debug().Interface("globe", g).Msg("Globe config")
	return g
}

func (g *Globe) Init(asyncLevel int, searchPaths []string, applicationNames []string) error {
	g.SearchPaths = searchPaths
	g.ApplicationNames = applicationNames

	g.collectEnvironments(searchPaths)

	return process(asyncLevel, g.environments, func(item interface{}) error {
		env, ok := item.(*Environment)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Environment")
		}
		return env.Init(applicationNames)
	})
}

func (g *Globe) Sync(asyncLevel int) error {
	vendirSecrets, err := g.generateVendirSecretYamls()
	if err != nil {
		return err
	}
	return process(asyncLevel, g.environments, func(item interface{}) error {
		env, ok := item.(*Environment)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Environment")
		}
		return env.Sync(asyncLevel, vendirSecrets)
	})
}

func (g *Globe) Render(asyncLevel int) error {
	return process(asyncLevel, g.environments, func(item interface{}) error {
		env, ok := item.(*Environment)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Environment")
		}
		return env.Render(asyncLevel)
	})
}

func (g *Globe) SyncAndRender(asyncLevel int) error {
	vendirSecrets, err := g.generateVendirSecretYamls()
	if err != nil {
		return err
	}
	return process(asyncLevel, g.environments, func(item interface{}) error {
		env, ok := item.(*Environment)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Environment")
		}
		return env.SyncAndRender(asyncLevel, vendirSecrets)
	})
}

// Cleanup discovers rendered environments that are not known to the Globe struct and removes them.
// This function should be only run when the Globe is not restricted by a list of environments.
func (g *Globe) Cleanup() error {
	legalEnvs := map[string]bool{}
	for _, env := range g.environments {
		legalEnvs[env.Id] = true
	}

	for _, dir := range [...]string{"argocd", "envs"} {
		dirPath := filepath.Join(g.RootDir, g.RenderedDir, dir)
		files, err := os.ReadDir(dirPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Debug().Str("dir", dirPath).Msg("Skipping cleanup of non-existing directory")
				continue
			}
			return fmt.Errorf("Unable to read dir: %w", err)
		}

		for _, file := range files {
			_, ok := legalEnvs[file.Name()]
			if file.IsDir() && !ok {
				log.Debug().Str("dir", dir+"/"+file.Name()).Msg("Cleanup rendered environment directory")
				fullPath := filepath.Join(dirPath, file.Name())
				err = os.RemoveAll(fullPath)
				if err != nil {
					log.Warn().Str("dir", fullPath).Msg("Failed to remove directory")
				}
			}
		}
	}

	return nil
}

// dumpConfigAsYaml dumps the globe config as yaml to a file and returns the file name
func (g *Globe) dumpConfigAsYaml() (string, error) {
	configData := struct {
		Myks *YttGlobeData `yaml:"myks"`
	}{
		Myks: &YttGlobeData{
			GitRepoBranch: g.GitRepoBranch,
			GitRepoUrl:    g.GitRepoUrl,
		},
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
				env, err := NewEnvironment(g, path)
				if err == nil {
					g.environments[path] = env
				} else {
					log.Debug().
						Err(err).
						Str("path", path).
						Msg("Unable to collect environment, might be base or parent environment. Skipping")
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to walk environment directories")
	}
}

func (g *Globe) setGitPathPrefix() error {
	if g.GitPathPrefix == "" {
		gitArgs := []string{}
		if g.RootDir != "" {
			gitArgs = append(gitArgs, "-C", g.RootDir)
		}
		gitArgs = append(gitArgs, "rev-parse", "--show-prefix")
		result, err := runCmd("git", nil, gitArgs, func(name string, args []string) {
			log.Debug().Msg(msgRunCmd("set git path prefix", name, args))
		})
		if err != nil {
			return err
		}
		g.GitPathPrefix = strings.Trim(result.Stdout, "\n")
	}
	return nil
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
