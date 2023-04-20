package kwhoosh

import (
	"os"
	"path/filepath"

	"github.com/creasty/defaults"
	"github.com/rs/zerolog/log"
)

// Define the main structure
type Kwhoosh struct {
	// Project root directory
	RootDir string
	// Base directory for environments
	EnvironmentBaseDir string `default:"envs"`
	// Prefix for kubernetes namespaces
	NamespacePrefix string `default:""`
	// ArgoCD namespace
	ArgoCDNamespace string `default:"argocd"`
	// Application data file name
	ApplicationDataFileName string `default:"app-data.ytt.yaml"`
	// Environment data file name
	EnvironmentDataFileName string `default:"env-data.ytt.yaml"`
	// Environment manfiest file name
	EnvironmentManifestFileName string `default:"manifest.ytt.yaml"`
	// Rendered environment manifest file name
	RenderedEnvironmentManifestFileName string `default:"rendered.manifest.yaml"`
	// Rendered vendir config file name
	RenderedVendirConfigFileName string `default:"rendered/vendir.yaml"`
	// Rendered vendir lock file name
	RenderedVendirLockFileName string `default:"rendered/vendir.lock.yaml"`

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

// CollectEnvironments scans the root directory for environment directories and
// collects them into the Kwhoosh struct.
// An environment directory is a directory that contains a file named after
// the EnvironmentDataFileName field (default: env-data.ytt.yaml).
// CollectEnvironments accepts a list of paths to scan. If the list is empty,
// the root directory is scanned.
// TODO: Better documentation
func (k *Kwhoosh) CollectEnvironments(searchPaths []string) {
	if len(searchPaths) == 0 {
		searchPaths = []string{k.EnvironmentBaseDir}
	}

	for _, searchPath := range searchPaths {
		k.collectEnvironments(searchPath)
	}

	log.Debug().Interface("environments", k.environments).Msg("Collected environments")
}

func (k *Kwhoosh) collectEnvironments(searchPath string) {
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
