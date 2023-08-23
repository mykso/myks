package myks

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

//go:embed templates/vendir/secret.ytt.yaml
var vendirSecretTemplate []byte

const GlobalLogFormat = "\033[1m[global]\033[0m %s"

var ErrNotClean = fmt.Errorf("target directory is not clean, aborting")

// Define the main structure
type Globe struct {
	/// Globe configuration

	// Base directory for environments
	EnvironmentBaseDir string `default:"envs"`
	// Prefix for kubernetes namespaces"
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
	// Main branch name
	MainBranchName string `default:"main"`

	/// User input

	// Application names to process
	ApplicationNames []string
	// Paths to scan for environments
	SearchPaths []string

	/// Runtime data

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

	if err := g.setGitRepoUrl(); err != nil {
		log.Warn().Err(err).Msg("Unable to set git repo url")
	}

	if err := g.setGitRepoBranch(); err != nil {
		log.Warn().Err(err).Msg("Unable to set git repo branch")
	}
	log.Debug().Interface("globe", g).Msg("Globe config")
	return g
}

func (g *Globe) Init(asyncLevel int, searchPaths []string, applicationNames []string) error {
	g.SearchPaths = searchPaths
	g.ApplicationNames = applicationNames

	yttLibraryDir := filepath.Join(g.RootDir, g.YttLibraryDirName)
	if _, err := os.Stat(yttLibraryDir); err == nil {
		g.extraYttPaths = append(g.extraYttPaths, yttLibraryDir)
	}

	dataSchemaFileName := filepath.Join(g.RootDir, g.ServiceDirName, g.TempDirName, g.DataSchemaFileName)
	if _, err := os.Stat(dataSchemaFileName); err != nil {
		log.Warn().Msg("Unable to find data schema file, creating one")
		if err := os.MkdirAll(filepath.Dir(dataSchemaFileName), 0o750); err != nil {
			log.Fatal().Err(err).Msg("Unable to create data schema file directory")
		}
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

// Bootstrap creates the initial directory structure and files
func (g *Globe) Bootstrap(force bool) error {
	log.Info().Msg("Creating base file structure")
	if err := g.createBaseFileStructure(force); err != nil {
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

func (g *Globe) createBaseFileStructure(force bool) error {
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

	if !force {
		if _, err := os.Stat(envDir); err == nil {
			return ErrNotClean
		}
		if _, err := os.Stat(protoDir); err == nil {
			return ErrNotClean
		}
		if _, err := os.Stat(renderedDir); err == nil {
			return ErrNotClean
		}
		if _, err := os.Stat(dataSchemaFile); err == nil {
			return ErrNotClean
		}
		if _, err := os.Stat(envsGitignoreFile); err == nil {
			return ErrNotClean
		}
	}

	if err := os.MkdirAll(envDir, 0o750); err != nil {
		return err
	}

	if err := os.MkdirAll(protoDir, 0o750); err != nil {
		return err
	}

	if err := os.MkdirAll(renderedDir, 0o750); err != nil {
		return err
	}

	if err := os.WriteFile(dataSchemaFile, dataSchema, 0o600); err != nil {
		return err
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
			if path != g.EnvironmentBaseDir {
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

func (g *Globe) collectVendirSecrets() map[string]*VendirCredentials {
	vendirCredentials := make(map[string]*VendirCredentials)

	usrRgx := regexp.MustCompile("^" + g.VendirSecretEnvPrefix + "(.+)_USERNAME=(.*)$")
	pswRgx := regexp.MustCompile("^" + g.VendirSecretEnvPrefix + "(.+)_PASSWORD=(.*)$")

	envvars := os.Environ()
	// Sort envvars to produce deterministic output for testing
	sort.Strings(envvars)
	for _, envPair := range envvars {
		if usrRgx.MatchString(envPair) {
			match := usrRgx.FindStringSubmatch(envPair)
			secretName := strings.ToLower(match[1])
			username := match[2]
			if vendirCredentials[secretName] == nil {
				vendirCredentials[secretName] = &VendirCredentials{}
			}
			vendirCredentials[secretName].Username = username
		} else if pswRgx.MatchString(envPair) {
			match := pswRgx.FindStringSubmatch(envPair)
			secretName := strings.ToLower(match[1])
			password := match[2]
			if vendirCredentials[secretName] == nil {
				vendirCredentials[secretName] = &VendirCredentials{}
			}
			vendirCredentials[secretName].Password = password
		}
	}

	for secretName, credentials := range vendirCredentials {
		if credentials.Username == "" || credentials.Password == "" {
			log.Warn().Msg("Incomplete credentials for secret: " + secretName)
			delete(vendirCredentials, secretName)
		}
	}

	var secretNames []string
	for secretName := range vendirCredentials {
		secretNames = append(secretNames, secretName)
	}
	log.Debug().Msg(g.Msg("Found vendir secrets: " + strings.Join(secretNames, ", ")))

	return vendirCredentials
}

func (g *Globe) generateVendirSecretYamls() (string, error) {
	vendirCredentials := g.collectVendirSecrets()

	var secretYamls string
	for secretName, credentials := range vendirCredentials {
		secretYaml, err := g.generateVendirSecretYaml(secretName, credentials.Username, credentials.Password)
		if err != nil {
			return secretYamls, err
		}
		secretYamls += "---\n" + secretYaml
	}

	return secretYamls, nil
}

func (g *Globe) generateVendirSecretYaml(secretName string, username string, password string) (string, error) {
	res, err := runYttWithFilesAndStdin(
		nil,
		bytes.NewReader(vendirSecretTemplate),
		func(name string, args []string) {
			log.Debug().Msg(g.Msg(msgRunCmd("render vendir secret yaml", name, args)))
		},
		"--data-value=secret_name="+secretName,
		"--data-value=username="+username,
		"--data-value=password="+password,
	)
	if err != nil {
		log.Error().Err(err).Msg(g.Msg(res.Stderr))
		return "", err
	}

	return res.Stdout, nil
}
