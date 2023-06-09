package myks

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/creasty/defaults"
	"github.com/rs/zerolog/log"
)

// Define the main structure
type Globe struct {
	/// Globe configuration

	// Project root directory
	RootDir string
	// Base directory for environments
	EnvironmentBaseDir string `default:"envs"`
	// Application prototypes directory
	PrototypesDir string `default:"prototypes"`
	// Rendered kubernetes manifests directory
	RenderedDir string `default:"rendered"`
	// Prefix for kubernetes namespaces
	NamespacePrefix string `default:""`

	/// Globe constants

	// Service directory name
	ServiceDirName string `default:".myks"`
	// Temporary directory name
	TempDirName string `default:"tmp"`
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
	// Helm charts directory name
	HelmChartsDirName string `default:"charts"`
	// Ytt step directory name
	YttStepDirName string `default:"ytt"`
	// Ytt library directory name
	YttLibraryDirName string `default:"lib"`

	/// User input

	// Paths to scan for environments
	SearchPaths []string
	// Application names to process
	ApplicationNames []string

	/// Runtime data

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

	yttLibraryDir := filepath.Join(g.RootDir, g.YttLibraryDirName)
	if _, err := os.Stat(yttLibraryDir); err == nil {
		g.extraYttPaths = append(g.extraYttPaths, yttLibraryDir)
	}

	log.Debug().Interface("globe", g).Msg("Globe config")
	return g
}

func (g *Globe) Init(searchPaths []string, applicationNames []string) error {
	g.SearchPaths = searchPaths
	g.ApplicationNames = applicationNames

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

func (g *Globe) collectEnvironments(searchPaths []string) {
	if len(searchPaths) == 0 {
		searchPaths = []string{g.EnvironmentBaseDir}
	}

	for _, searchPath := range searchPaths {
		g.collectEnvironmentsInPath(searchPath)
	}

	log.Debug().Interface("environments", g.environments).Msg("Collected environments")
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

func (g *Globe) ytt(paths []string, args ...string) (CmdResult, error) {
	return g.yttS(paths, nil, args...)
}

func (g *Globe) yttS(paths []string, stdin io.Reader, args ...string) (CmdResult, error) {
	return runYttWithFilesAndStdin(append(g.extraYttPaths, paths...), stdin, args...)
}
