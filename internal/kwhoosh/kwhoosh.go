package kwhoosh

import (
	"os"
	"path/filepath"

	"github.com/creasty/defaults"
	"github.com/rs/zerolog/log"
)

// Define the main structure
type Kwhoosh struct {
	/// Kwhoosh configuration

	// Project root directory
	RootDir string
	// Base directory for environments
	EnvironmentBaseDir string `default:"envs"`
	// Application prototypes directory
	PrototypesDir string `default:"prototypes"`
	// Prefix for kubernetes namespaces
	NamespacePrefix string `default:""`
	// ArgoCD namespace
	ArgoCDNamespace string `default:"argocd"`

	/// Kwhoosh constants

	// Service directory name
	ServiceDirName string `default:".kwhoosh"`
	// Application data file name
	ApplicationDataFileName string `default:"app-data.ytt.yaml"`
	// Environment data file name
	EnvironmentDataFileName string `default:"env-data.ytt.yaml"`
	// Environment manfiest template file name
	EnvironmentManifestTemplateFileName string `default:"manifest.ytt.yaml"`
	// Rendered environment manifest file name
	EnvironmentManifestFileName string `default:"manifest.yaml"`
	// Rendered vendir config file name
	VendirConfigFileName string `default:"vendir.yaml"`
	// Rendered vendir lock file name
	VendirLockFileName string `default:"vendir.lock.yaml"`
	// Downloaded third-party sources
	VendorDirName string `default:"vendor"`

	/// User input

	// Paths to scan for environments
	SearchPaths []string
	// Application names to process
	ApplicationNames []string

	/// Runtime data

	// Collected environments for processing
	environments map[string]*Environment
}

func New(rootDir string) *Kwhoosh {
	k := &Kwhoosh{
		RootDir:      rootDir,
		environments: make(map[string]*Environment),
	}
	if err := defaults.Set(k); err != nil {
		log.Fatal().Err(err).Msg("Unable to set defaults")
	}
	log.Debug().Interface("kwhoosh", k).Msg("Kwhoosh config")
	return k
}

func (k *Kwhoosh) Init(searchPaths []string, applicationNames []string) error {
	k.SearchPaths = searchPaths
	k.ApplicationNames = applicationNames

	k.collectEnvironments(searchPaths)

	for _, env := range k.environments {
		if err := env.Init(applicationNames); err != nil {
			return err
		}
	}

	return nil
}

func (k *Kwhoosh) Sync() error {
	for _, env := range k.environments {
		if err := env.Sync(); err != nil {
			return err
		}
	}
	return nil
}

func (k *Kwhoosh) collectEnvironments(searchPaths []string) {
	if len(searchPaths) == 0 {
		searchPaths = []string{k.EnvironmentBaseDir}
	}

	for _, searchPath := range searchPaths {
		k.collectEnvironmentsInPath(searchPath)
	}

	log.Debug().Interface("environments", k.environments).Msg("Collected environments")
}

func (k *Kwhoosh) collectEnvironmentsInPath(searchPath string) {
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			_, err := os.Stat(filepath.Join(path, k.EnvironmentDataFileName))
			if err == nil {
				env := NewEnvironment(k, path)
				if env != nil {
					k.environments[filepath.Dir(path)] = env
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
