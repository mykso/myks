// Package myks provides all the functionality for myks.
package myks

import (
	"bytes"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/creasty/defaults"
	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

const GlobalLogFormat = "\033[1m[global]\033[0m %s"

const GlobalExtendedLogFormat = "\033[1m[global > %s > %s]\033[0m %s"

// Globe contains global configuration and state for the myks application
type Globe struct {
	// Global vendir cache dir
	VendirCache string `default:"vendir-cache"`
	// Project root directory
	RootDir string `default:"."`
	// Base directory for environments
	EnvironmentBaseDir string `default:"envs"`
	// Application prototypes directory
	PrototypesDir string `default:"prototypes"`
	// Ytt library directory name
	YttLibraryDirName string `default:"lib"`
	// Rendered kubernetes manifests directory
	RenderedEnvsDir string `default:"rendered/envs"`
	// Rendered argocd manifests directory
	RenderedArgoDir string `default:"rendered/argocd"`

	// Directory of application-specific configuration
	AppsDir string `default:"_apps"`
	// Directory of environment-specific configuration
	EnvsDir string `default:"_env"`
	// Directory of application-specific prototype overwrites
	PrototypeOverrideDir string `default:"_proto"`

	// Data values schema file name
	DataSchemaFileName string `default:"data-schema.ytt.yaml"`
	// Application data file name
	ApplicationDataFileName string `default:"app-data*.yaml"`
	// Environment data file name
	EnvironmentDataFileName string `default:"env-data*.yaml"`
	// Rendered environment data file name
	RenderedEnvironmentDataFileName string `default:"env-data.yaml"`
	// Myks runtime data file name
	MyksDataFileName string `default:"myks-data.ytt.yaml"`
	// Service directory name
	ServiceDirName string `default:".myks"`
	// Temporary directory name
	TempDirName string `default:"tmp"`

	// Rendered vendir config file name
	VendirConfigFileName string `default:"vendir.yaml"`
	// Rendered vendir lock file name
	VendirLockFileName string `default:"vendir.lock.yaml"`
	// Name of the file with directory-to-cache-dir mappings
	VendirLinksMapFileName string `default:"vendir-links.yaml"`
	// Prefix for vendir secret environment variables
	VendirSecretEnvPrefix string `default:"VENDIR_SECRET_"`

	// Downloaded third-party sources
	VendorDirName string `default:"vendor"`
	// Helm charts directory name
	HelmChartsDirName string `default:"charts"`

	// Plugin subdirectories
	// ArgoCD data directory name
	ArgoCDDataDirName string `default:"argocd"`
	// Helm step directory name
	HelmStepDirName string `default:"helm"`
	// Static files directory name
	StaticFilesDirName string `default:"static"`
	// Vendir step directory name
	VendirStepDirName string `default:"vendir"`
	// Ytt step directory name
	YttPkgStepDirName string `default:"ytt-pkg"`
	// Ytt step directory name
	YttStepDirName string `default:"ytt"`

	/// Runtime data

	// Running in a git repository
	WithGit bool
	// Git repository path prefix (non-empty if running in a subdirectory of a git repository)
	GitPathPrefix string
	// Git repository branch
	GitRepoBranch string
	// Git repository URL
	GitRepoURL string

	// Prefix for kubernetes namespaces, only used in helm rendering
	NamespacePrefix string `default:""`

	// Collected environments for processing
	environments map[string]*Environment

	// Extra ytt file paths
	extraYttPaths []string
}

// YttGlobeData controls runtime data available to ytt templates
type YttGlobeData struct {
	GitRepoBranch string `yaml:"gitRepoBranch"`
	GitRepoURL    string `yaml:"gitRepoUrl"`
}

type VendirCredentials struct {
	Username string
	Password string
}

type EnvAppMap map[string][]string

func NewWithDefaults() *Globe {
	g := &Globe{}
	if err := defaults.Set(g); err != nil {
		log.Fatal().Err(err).Msg("Unable to set defaults")
	}
	return g
}

func New(rootDir string) *Globe {
	// FIXME: Do not change working directory here, implement relative paths throughout the codebase instead
	if rootDir != "." {
		if err := os.Chdir(rootDir); err != nil {
			log.Fatal().Err(err).Str("rootDir", rootDir).Msg("Unable to change working directory")
		}
		rootDir = "."
	}
	g := NewWithDefaults()
	g.RootDir = rootDir
	g.environments = make(map[string]*Environment)

	g.initGitData()

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

func (g *Globe) initGitData() {
	if isGitRepo(g.RootDir) {
		g.WithGit = true

		if gitPathPrefix, err := getGitPathPrefix(g.RootDir); err != nil {
			log.Warn().Err(err).Msg("Unable to set git path prefix")
		} else {
			g.GitPathPrefix = gitPathPrefix
		}

		if gitRepoBranch, err := getGitRepoBranch(g.RootDir); err != nil {
			log.Warn().Err(err).Msg("Unable to set git repo url")
		} else {
			g.GitRepoBranch = gitRepoBranch
		}

		if gitRepoURL, err := getGitRepoURL(g.RootDir); err != nil {
			log.Warn().Err(err).Msg("Unable to set git repo branch")
		} else {
			g.GitRepoURL = gitRepoURL
		}
	} else {
		log.Warn().Msg("Not in a git repository, Smart Mode and git-related data will not be available")
	}
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
	envAppMap := g.collectEnvironments(g.AddBaseDirToEnvAppMap(envSearchPathToAppMap))
	log.Debug().Interface("envAppMap", envAppMap).Msg(g.Msg("Environments collected from search paths"))

	return process(asyncLevel, maps.Keys(envAppMap), func(envPath string) error {
		env, ok := g.environments[envPath]
		if !ok {
			return fmt.Errorf("unable to find environment for path: %s", envPath)
		}
		appNames, ok := envAppMap[envPath]
		if !ok {
			return fmt.Errorf("unable to find app names for path: %s", envPath)
		}
		return env.Init(appNames)
	})
}

func (g *Globe) Sync(asyncLevel int) error {
	syncTools := g.getSyncTools()
	for _, syncTool := range syncTools {
		secrets, err := syncTool.GenerateSecrets(g)
		if err != nil {
			return err
		}
		err = process(asyncLevel, maps.Values(g.environments), func(env *Environment) error {
			return env.Sync(asyncLevel, syncTool, secrets)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *Globe) Render(asyncLevel int) error {
	return process(asyncLevel, maps.Values(g.environments), func(env *Environment) error {
		return env.Render(asyncLevel)
	})
}

func (g *Globe) SyncAndRender(asyncLevel int) error {
	err := g.Sync(asyncLevel)
	if err != nil {
		return err
	}
	return g.Render(asyncLevel)
}

// ExecPlugin executes a plugin in the context of the globe
func (g *Globe) ExecPlugin(asyncLevel int, p Plugin, args []string) error {
	return process(asyncLevel, maps.Values(g.environments), func(env *Environment) error {
		return env.ExecPlugin(asyncLevel, p, args)
	})
}

// CleanupRenderedManifests discovers rendered environments that are not known to the Globe struct and removes them.
// This function should be only run when the Globe is not restricted by a list of environments.
func (g *Globe) CleanupRenderedManifests(dryRun bool) error {
	legalEnvs := map[string]*Environment{}
	for _, env := range g.environments {
		legalEnvs[env.ID] = env
	}

	listFiles := func(dir string) ([]fs.DirEntry, error) {
		files, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				log.Debug().Str("dir", dir).Msg("Skipping cleanup of non-existing directory")
				return nil, nil
			}
			return nil, fmt.Errorf("unable to read directory %s: %w", dir, err)
		}
		return files, nil
	}

	cleanupEnvironmentDir := func(root string, envDirEntry fs.DirEntry, getAppNameFunc func(string) string) {
		envID := envDirEntry.Name()
		fullPath := filepath.Join(root, envID)
		if !envDirEntry.IsDir() {
			log.Warn().Str("file", fullPath).Msg("Skipping non-directory entry")
			return
		}

		env, ok := legalEnvs[envID]
		if !ok {
			if dryRun {
				log.Info().Str("dir", fullPath).Msg("Would cleanup rendered environment directory")
				return
			}
			log.Debug().Str("dir", fullPath).Msg("Cleanup rendered environment directory")
			if err := os.RemoveAll(fullPath); err != nil {
				log.Warn().Str("dir", fullPath).Msg("Failed to remove directory")
			}
			return
		}

		legalApps := map[string]bool{}
		for _, app := range env.Applications {
			legalApps[app.Name] = true
		}

		apps, err := listFiles(fullPath)
		if err != nil {
			log.Warn().Err(err).Str("dir", fullPath).Msg("Unable to list applications in environment directory")
			return
		}

		for _, appDirEntry := range apps {
			appName := appDirEntry.Name()
			fullAppPath := filepath.Join(fullPath, appName)
			if getAppNameFunc != nil {
				appName = getAppNameFunc(appName)
			}
			if appName == "" {
				log.Warn().Str("app", fullAppPath).Msg("Directory name could not be mapped to a known application")
				continue
			}
			if _, ok := legalApps[appName]; !ok {
				if dryRun {
					log.Info().Str("app", fullAppPath).Msg("Would cleanup rendered application directory")
					continue
				}
				log.Debug().Str("app", fullAppPath).Msg("Cleanup rendered application directory")
				if err := os.RemoveAll(fullAppPath); err != nil {
					log.Warn().Str("app", fullAppPath).Msg("Failed to remove application directory")
				}
			}
		}
	}

	argoDir := filepath.Join(g.RootDir, g.RenderedArgoDir)
	files, err := listFiles(argoDir)
	if err != nil {
		return fmt.Errorf("unable to read ArgoCD rendered manifests directory: %w", err)
	}
	for _, envDirEntry := range files {
		cleanupEnvironmentDir(argoDir, envDirEntry, func(appName string) string {
			if strings.HasPrefix(appName, "app-") && strings.HasSuffix(appName, ".yaml") {
				return appName[4 : len(appName)-5]
			}
			return ""
		})
	}

	envsDir := filepath.Join(g.RootDir, g.RenderedEnvsDir)
	files, err = listFiles(envsDir)
	if err != nil {
		return fmt.Errorf("unable to read rendered environments directory: %w", err)
	}
	for _, envDirEntry := range files {
		cleanupEnvironmentDir(envsDir, envDirEntry, nil)
	}

	return nil
}

// CleanupObsoleteCacheEntries removes cache entries that are not used by any application.
// This function should be only run when the Globe is not restricted by a list of environments.
func (g *Globe) CleanupObsoleteCacheEntries(dryRun bool) error {
	validCacheDirs := map[string]bool{}
	for _, env := range g.environments {
		for _, app := range env.Applications {
			linksMap, err := app.getLinksMap()
			if err != nil {
				return err
			}
			for _, cacheName := range linksMap {
				validCacheDirs[cacheName] = true
			}
		}
	}

	cacheDir := filepath.Join(g.RootDir, g.ServiceDirName, g.VendirCache)
	cacheEntries, err := os.ReadDir(cacheDir)
	if os.IsNotExist(err) {
		log.Debug().Str("dir", cacheDir).Msg("Skipping cleanup of non-existing directory")
		return nil
	} else if err != nil {
		return fmt.Errorf("unable to read dir: %w", err)
	}

	for _, entry := range cacheEntries {
		if !entry.IsDir() {
			log.Warn().Str("file", cacheDir+"/"+entry.Name()).Msg("Skipping non-directory entry")
			continue
		}
		if _, ok := validCacheDirs[entry.Name()]; !ok {
			if dryRun {
				log.Info().Str("dir", cacheDir+"/"+entry.Name()).Msg("Would cleanup cache entry")
				continue
			}
			log.Debug().Str("dir", cacheDir+"/"+entry.Name()).Msg("Cleanup cache entry")
			fullPath := filepath.Join(cacheDir, entry.Name())
			if err = os.RemoveAll(fullPath); err != nil {
				log.Warn().Str("dir", fullPath).Msg("Failed to remove directory")
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
			GitRepoURL:    g.GitRepoURL,
		},
	}
	var yamlData bytes.Buffer
	enc := yaml.NewEncoder(&yamlData)
	enc.SetIndent(2)
	if err := enc.Encode(configData); err != nil {
		return "", err
	}
	yttData := fmt.Sprintf("#@data/values-schema\n---\n%s", yamlData.String())

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
			// If appNames is nil or empty, it means all applications in the environment should be processed
			if len(appNames) == 0 {
				envAppMap[envPath] = nil
				continue
			}
			// If the environment is already in the map, append the appNames to the existing list,
			// but only if its apps are not nil
			if apps, ok := envAppMap[envPath]; !ok {
				envAppMap[envPath] = appNames
			} else if apps != nil {
				envAppMap[envPath] = append(apps, appNames...)
			}
		}
	}
	for env, apps := range envAppMap {
		envAppMap[env] = unique(apps)
	}

	log.Debug().Interface("envToAppMap", envAppMap).Msg(g.Msg("Collected environments"))
	return envAppMap
}

func (g *Globe) collectEnvironmentsInPath(searchPath string) []string {
	result := []string{}
	searchPath = filepath.Join(g.RootDir, searchPath)
	err := filepath.WalkDir(filepath.Clean(searchPath), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d != nil && d.IsDir() {
			files, err := filepath.Glob(filepath.Join(path, g.EnvironmentDataFileName))
			if err != nil {
				return err
			}
			if len(files) == 0 {
				return nil
			}
			// Try all files in the directory until a valid environment is found
			for _, file := range files {
				env, err := NewEnvironment(g, path, file)
				if err == nil {
					g.environments[path] = env
					result = append(result, path)
					return nil
				}
			}
			log.Debug().
				Str("path", path).
				Strs("files", files).
				Msg("Unable to collect environment, might be base or parent environment. Skipping")

		}
		return nil
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to walk environment directories")
	}
	return result
}

func (g *Globe) Msg(msg string) string {
	formattedMessage := fmt.Sprintf(GlobalLogFormat, msg)
	return formattedMessage
}

func (g *Globe) getSyncTools() []SyncTool {
	syncTools := []SyncTool{
		&VendirSyncer{ident: "vendir"},
		&HelmSyncer{ident: "helm"},
	}
	return syncTools
}

func (g *Globe) GetEnvs() map[string]*Environment {
	return g.environments
}

func (g *Globe) AddBaseDirToEnvAppMap(envSearchPathToAppMap EnvAppMap) EnvAppMap {
	envAppMap := EnvAppMap{}
	for envPath, val := range envSearchPathToAppMap {
		envAppMap[g.AddBaseDirToEnvPath(envPath)] = val
	}
	return envAppMap
}

// AddBaseDirToEnvPath adds the base directory (Globe.EnvironmentBaseDir) to the environment path if it is not already present.
func (g *Globe) AddBaseDirToEnvPath(envName string) string {
	if envName == g.EnvironmentBaseDir {
		return envName
	}
	if strings.HasPrefix(envName, g.EnvironmentBaseDir+string(filepath.Separator)) {
		return envName
	}
	return filepath.Join(g.EnvironmentBaseDir, envName)
}

func (g *Globe) getEnvByID(envID string) (*Environment, error) {
	for _, env := range g.environments {
		if env.ID == envID {
			return env, nil
		}
	}
	return nil, fmt.Errorf("environment with ID %s not found", envID)
}
