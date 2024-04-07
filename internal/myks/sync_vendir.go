package myks

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

// This string defines an overlay that prefixes paths of all vendir directories with the vendor directory name.
// This makes the paths relative to the root directory, allowing to use other relative paths as sources for vendir.
//
// Despite vendir disallows using multiple config files, by using `expects="1+"` we explicitly allow to have multiple
// of them on the config generation step. In case there are multiple vendir configs, processing will fail on the vendir
// sync step. This will provide a better error message for the user.
const vendorDirOverlayTemplate = `
#@ load("@ytt:overlay", "overlay")
#@overlay/match by=overlay.subset({"kind": "Config", "apiVersion": "vendir.k14s.io/v1alpha1"}), expects="1+"
---
directories:
  - #@overlay/match by=overlay.all, expects="1+"
    #@overlay/replace via=lambda l, r: (r + "/" + l).replace("//", "/")
    path: %s`

type CacheConfig struct {
	Enabled bool
}

type VendirSyncer struct {
	ident string
}

func (v *VendirSyncer) Ident() string {
	return v.ident
}

func (v *VendirSyncer) Sync(a *Application, vendirSecrets string) error {
	if err := v.prepareSync(a); err != nil {
		if err == ErrNoVendirConfig {
			log.Info().Msg(a.Msg(v.getStepName(), "No vendir config found"))
			return nil
		}
		return err
	}

	if err := v.doSync(a, vendirSecrets); err != nil {
		return err
	}
	return nil
}

// creates vendir yaml file
func (v *VendirSyncer) prepareSync(a *Application) error {
	// Collect ytt arguments following the following steps:
	// 1. If exists, use the `apps/<prototype>/vendir` directory.
	// 2. If exists, for every level of environments use `<env>/_apps/<app>/vendir` directory.

	var yttFiles []string

	protoVendirDir := filepath.Join(a.Prototype, "vendir")
	if ok, err := isExist(protoVendirDir); err != nil {
		return err
	} else if ok {
		yttFiles = append(yttFiles, protoVendirDir)
	}

	appVendirDirs := a.e.collectBySubpath(filepath.Join(a.e.g.AppsDir, a.Name, "vendir"))
	yttFiles = append(yttFiles, appVendirDirs...)

	if len(yttFiles) == 0 {
		return ErrNoVendirConfig
	}

	// create vendir config yaml
	vendirConfig, err := a.yttS(v.getStepName(), "creating vendir config", yttFiles, nil)
	if err != nil {
		return err
	}

	if vendirConfig.Stdout == "" {
		err = errors.New("Empty vendir config")
		return err
	}

	vendirConfigFilePath := a.expandServicePath(a.e.g.VendirConfigFileName)
	// Create directory if it does not exist
	err = createDirectory(filepath.Dir(vendirConfigFilePath))
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to create directory for vendir config file"))
		return err
	}
	err = os.WriteFile(vendirConfigFilePath, []byte(vendirConfig.Stdout), 0o600)
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to write vendir config file"))
		return err
	}

	return nil
}

func (v *VendirSyncer) doSync(a *Application, vendirSecrets string) error {
	vendirConfigPath := a.expandServicePath(a.e.g.VendirConfigFileName)
	vendirPatchedConfigPath := a.expandServicePath(a.e.g.VendirPatchedConfigFileName)
	vendirLockFilePath := a.expandServicePath(a.e.g.VendirLockFileName)

	// read vendir config
	vendirConfig, err := unmarshalYamlToMap(vendirConfigPath)
	if err != nil {
		return err
	}
	cacheConfig, err := v.getCacheConfig(a)

	for _, dir := range vendirConfig["directories"].([]interface{}) {
		dirMap := dir.(map[string]interface{})
		dirPath := dirMap["path"].(string)
		// iterate over vendir contents
		for _, content := range dirMap["contents"].([]interface{}) {
			contentMap := content.(map[string]interface{})
			if err != nil {
				return err
			}
			path := filepath.Join(dirPath, contentMap["path"].(string))
			cacheName, err := findCacheNamer(contentMap).Name(path, contentMap)
			if err != nil {
				return err
			}
			outputPath := a.expandVendirCache(cacheName)
			overlayReader := strings.NewReader(fmt.Sprintf(vendorDirOverlayTemplate, outputPath))
			yttFiles := []string{vendirConfigPath}
			vendirConfig, err := a.yttS(v.getStepName(), "creating final vendir config", yttFiles, overlayReader)
			if err != nil {
				return err
			}
			err = writeFile(vendirPatchedConfigPath, []byte(vendirConfig.Stdout))
			if err != nil {
				return err
			}
			lazy := true
			if contentMap["lazy"] != nil {
				lazy = contentMap["lazy"].(bool)
			}
			exists, _ := isExist(outputPath)
			if
			// run sync if cache does not exist in any case
			!exists ||
				// lazy has been explicitly set to false in vendir config
				!lazy ||
				// run sync if caching was disabled for app
				!cacheConfig.Enabled {
				if err := v.runVendirSync(a, vendirPatchedConfigPath, filepath.Join(outputPath, path), vendirLockFilePath, vendirSecrets); err != nil {
					log.Error().Err(err).Msg(a.Msg(v.getStepName(), "Vendir sync failed"))
					err := removeDirectory(outputPath)
					return err
				}
				// else, do nothing
			} else {
				log.Info().Msg(a.Msg(v.getStepName(), "Skipping vendir sync, cache exists"))
			}
		}
	}
	return nil
	// vendorDir := a.expandVendirCache(a.e.g.VendorDirName)
	// return v.cleanupVendorDir(a, vendorDir, vendirConfigPath)
}

func (v *VendirSyncer) runVendirSync(a *Application, vendirConfigPath, vendirConfigDirPath, vendirLock, vendirSecrets string) error {
	// TODO sync retry - maybe as vendir MR
	args := []string{
		"vendir",
		"sync",
		"--file=" + vendirConfigPath,
		"--lock-file=" + vendirLock,
		"--directory=" + vendirConfigDirPath,
		"--file=-",
	}
	_, err := a.runCmd(v.getStepName(), "vendir sync", "myks", strings.NewReader(vendirSecrets), args)
	if err != nil {
		return err
	}
	log.Info().Msg(a.Msg(v.getStepName(), "Synced"))
	return nil
}

func (v *VendirSyncer) getStepName() string {
	return fmt.Sprintf("%s-%s", syncStepName, v.Ident())
}

func (hr *VendirSyncer) getCacheConfig(a *Application) (CacheConfig, error) {
	dataValuesYaml, err := a.ytt(hr.getStepName(), "get cache config", a.yttDataFiles, "--data-values-inspect")
	if err != nil {
		return CacheConfig{}, err
	}

	var cacheConfig struct {
		Cache CacheConfig
	}
	err = yaml.Unmarshal([]byte(dataValuesYaml.Stdout), &cacheConfig)
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(hr.getStepName(), "Unable to unmarshal data values"))
		return CacheConfig{}, err
	}

	return cacheConfig.Cache, nil
}
