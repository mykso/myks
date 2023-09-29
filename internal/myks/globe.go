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
	"golang.org/x/exp/maps"
	yaml "gopkg.in/yaml.v3"
)

const GlobalLogFormat = "\033[1m[global]\033[0m %s"

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

type EnvAppMap map[string][]string

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
	if ok, err := isExist(yttLibraryDir); err != nil {
		log.Fatal().Err(err).Str("path", yttLibraryDir).Msg("Unable to stat ytt library directory")
	} else if ok {
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

// ValidateRootDir checks if the specified root directory contains required subdirectories
func (g *Globe) ValidateRootDir() error {
	for _, dir := range [...]string{g.EnvironmentBaseDir, g.PrototypesDir} {
		dirPath := filepath.Join(g.RootDir, dir)
		_, err := os.Stat(dirPath)
		if os.IsNotExist(err) {
			log.Warn().Str("dir", dir).Msg(g.Msg("Required directory does not exist. Did you run `myks init`?"))
			return err
		} else if err != nil {
			log.Error().Err(err).Str("dir", dir).Msg(g.Msg("Unable to stat directory"))
			return err
		}
	}

	return nil
}

func (g *Globe) Init(asyncLevel int, envSearchPathToAppMap EnvAppMap) error {
	envAppMap := g.collectEnvironments(envSearchPathToAppMap)

	return process(asyncLevel, maps.Keys(envAppMap), func(item interface{}) error {
		envPath, ok := item.(string)
		if !ok {
			return fmt.Errorf("Unable to cast item to string")
		}
		env, ok := g.environments[envPath]
		if !ok {
			return fmt.Errorf("Unable to find environment for path: %s", envPath)
		}
		appNames, ok := envAppMap[envPath]
		if !ok {
			return fmt.Errorf("Unable to find app names for path: %s", envPath)
		}
		return env.Init(appNames)
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

func (g *Globe) collectEnvironments(envSearchPathToAppMap EnvAppMap) EnvAppMap {
	envAppMap := EnvAppMap{}

	if len(envSearchPathToAppMap) == 0 {
		envSearchPathToAppMap = EnvAppMap{g.EnvironmentBaseDir: []string{}}
	}

	for searchPath, appNames := range envSearchPathToAppMap {
		for _, envPath := range g.collectEnvironmentsInPath(searchPath) {
			envAppMap[envPath] = appNames
		}
	}

	log.Debug().Interface("envToAppMap", envAppMap).Msg(g.Msg("Collected environments"))
	return envAppMap
}

func (g *Globe) collectEnvironmentsInPath(searchPath string) []string {
	result := []string{}
	err := filepath.WalkDir(filepath.Clean(searchPath), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d != nil && d.IsDir() {
			if ok, err := isExist(filepath.Join(path, g.EnvironmentDataFileName)); err != nil {
				return err
			} else if ok {
				env, err := NewEnvironment(g, path)
				if err == nil {
					g.environments[path] = env
					result = append(result, path)
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
	return result
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
