package myks

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

type Cleaner struct {
	g *Globe
}

func NewCleaner(g *Globe) *Cleaner {
	return &Cleaner{g: g}
}

// CleanupRenderedManifests discovers rendered environments that are not known to the Globe struct and removes them.
// This function should be only run when the Globe is not restricted by a list of environments.
func (c *Cleaner) CleanupRenderedManifests(dryRun bool) error {
	legalEnvs := c.getLegalEnvs()

	argoDir := filepath.Join(c.g.Config.RootDir, c.g.Config.RenderedArgoDir)
	files, err := c.listFiles(argoDir)
	if err != nil {
		return fmt.Errorf("unable to read ArgoCD rendered manifests directory: %w", err)
	}
	for _, envDirEntry := range files {
		c.cleanupEnvironmentDir(argoDir, envDirEntry, legalEnvs, dryRun, func(appName string) string {
			if strings.HasPrefix(appName, "app-") && strings.HasSuffix(appName, ".yaml") {
				return appName[4 : len(appName)-5]
			}
			return ""
		})
	}

	envsDir := filepath.Join(c.g.Config.RootDir, c.g.Config.RenderedEnvsDir)
	files, err = c.listFiles(envsDir)
	if err != nil {
		return fmt.Errorf("unable to read rendered environments directory: %w", err)
	}
	for _, envDirEntry := range files {
		c.cleanupEnvironmentDir(envsDir, envDirEntry, legalEnvs, dryRun, nil)
	}

	return nil
}

func (c *Cleaner) getLegalEnvs() map[string]*Environment {
	legalEnvs := map[string]*Environment{}
	for _, env := range c.g.environments {
		legalEnvs[env.ID] = env
	}
	return legalEnvs
}

func (c *Cleaner) listFiles(dir string) ([]fs.DirEntry, error) {
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

func (c *Cleaner) cleanupEnvironmentDir(root string, envDirEntry fs.DirEntry, legalEnvs map[string]*Environment, dryRun bool, getAppNameFunc func(string) string) {
	envID := envDirEntry.Name()
	fullPath := filepath.Join(root, envID)
	if !envDirEntry.IsDir() {
		log.Warn().Str("file", fullPath).Msg("Skipping non-directory entry")
		return
	}

	env, ok := legalEnvs[envID]
	if !ok {
		c.removeDir(fullPath, "environment", dryRun)
		return
	}

	c.cleanupApplicationDirs(fullPath, env, dryRun, getAppNameFunc)
}

func (c *Cleaner) cleanupApplicationDirs(fullPath string, env *Environment, dryRun bool, getAppNameFunc func(string) string) {
	legalApps := map[string]bool{}
	for _, app := range env.Applications {
		legalApps[app.Name] = true
	}

	apps, err := c.listFiles(fullPath)
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
		if !legalApps[appName] {
			c.removeDir(fullAppPath, "application", dryRun)
		}
	}
}

func (c *Cleaner) removeDir(fullPath string, dirType string, dryRun bool) {
	if dryRun {
		log.Info().Str("dir", fullPath).Msgf("Would cleanup rendered %s directory", dirType)
		return
	}
	log.Debug().Str("dir", fullPath).Msgf("Cleanup rendered %s directory", dirType)
	if err := os.RemoveAll(fullPath); err != nil {
		log.Warn().Str("dir", fullPath).Msg("Failed to remove directory")
	}
}

// CleanupObsoleteCacheEntries removes cache entries that are not used by any application.
// This function should be only run when the Globe is not restricted by a list of environments.
func (c *Cleaner) CleanupObsoleteCacheEntries(dryRun bool) error {
	validCacheDirs := map[string]bool{}
	for _, env := range c.g.environments {
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

	cacheDir := filepath.Join(c.g.Config.RootDir, c.g.Config.ServiceDirName, c.g.Config.VendirCache)
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
		if !validCacheDirs[entry.Name()] {
			c.removeDir(filepath.Join(cacheDir, entry.Name()), "cache entry", dryRun)
		}
	}

	return nil
}
