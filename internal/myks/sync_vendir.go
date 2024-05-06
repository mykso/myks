package myks

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

type VendirSyncer struct {
	ident string
}

func (v *VendirSyncer) Ident() string {
	return v.ident
}

func (v *VendirSyncer) Sync(a *Application, vendirSecrets string) error {
	if err := v.renderVendirConfig(a); err == ErrNoVendirConfig {
		log.Info().Msg(a.Msg(v.getStepName(), "No vendir config found"))
		return nil
	} else if err != nil {
		return err
	}

	if err := v.extractCacheItems(a); err != nil {
		return err
	}

	if err := v.doSync(a, vendirSecrets); err != nil {
		return err
	}
	log.Info().Msg(a.Msg(v.getStepName(), "Synced"))
	return nil
}

// creates vendir yaml file
func (v *VendirSyncer) renderVendirConfig(a *Application) error {
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
		return errors.New("rendered empty vendir config")
	}

	vendirConfigFilePath := a.expandServicePath(a.e.g.VendirConfigFileName)
	// Create directory if it does not exist
	if err := createDirectory(filepath.Dir(vendirConfigFilePath)); err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to create directory for vendir config file"))
		return err
	}

	if err := os.WriteFile(vendirConfigFilePath, []byte(vendirConfig.Stdout), 0o600); err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to write vendir config file"))
		return err
	}

	return nil
}

func (v *VendirSyncer) doSync(a *Application, vendirSecrets string) error {
	linksMapPath := a.getLinksMapPath()
	linksMap, err := unmarshalYamlToMap(linksMapPath)
	if err != nil {
		return err
	}

	for contentPath, cacheName := range linksMap {
		cacheDir := a.expandVendirCache(cacheName.(string))
		vendirConfigPath := filepath.Join(cacheDir, a.e.g.VendirConfigFileName)
		vendirLockPath := filepath.Join(cacheDir, a.e.g.VendirLockFileName)
		if err := v.runVendirSync(a, vendirConfigPath, vendirLockPath, vendirSecrets); err != nil {
			log.Error().Err(err).Msg(a.Msg(v.getStepName(), "Vendir sync failed"))
			return err
		}
		if err := v.linkVendorToCache(a, contentPath, cacheName.(string)); err != nil {
			log.Error().Err(err).Msg(a.Msg(v.getStepName(), "Unable to create link to cache"))
			return err
		}
	}

	return nil
	// vendorDir := a.expandVendirCache(a.e.g.VendorDirName)
	// return v.cleanupVendorDir(a, vendorDir, vendirConfigPath)
}

func (v *VendirSyncer) linkVendorToCache(a *Application, vendorPath, cacheName string) error {
	linkFullPath := a.expandVendorPath(vendorPath)
	linkDir := filepath.Dir(linkFullPath)
	cacheDataPath := path.Join(a.expandVendirCache(cacheName), "data")

	relCacheDataPath, err := filepath.Rel(linkDir, cacheDataPath)
	if err != nil {
		return err
	}

	if err := createDirectory(linkDir); err != nil {
		return err
	}

	if err := os.Remove(linkFullPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.Symlink(relCacheDataPath, linkFullPath)
}

func (v *VendirSyncer) runVendirSync(a *Application, vendirConfig, vendirLock, vendirSecrets string) error {
	// TODO sync retry - maybe as vendir MR
	args := []string{
		"vendir",
		"sync",
		"--file=" + vendirConfig,
		"--lock-file=" + vendirLock,
		"--file=-",
	}
	_, err := a.runCmd(v.getStepName(), "vendir sync", myksFullPath(), strings.NewReader(vendirSecrets), args)
	return err
}

func (v *VendirSyncer) getStepName() string {
	return fmt.Sprintf("%s-%s", syncStepName, v.Ident())
}

func (v *VendirSyncer) extractCacheItems(a *Application) error {
	vendirConfig, err := unmarshalYamlToMap(a.expandServicePath(a.e.g.VendirConfigFileName))
	if err != nil {
		return err
	}

	vendorDirToCacheMap := map[string]string{}

	for _, dir := range vendirConfig["directories"].([]interface{}) {
		dirMap := dir.(map[string]interface{})
		contents := dirMap["contents"].([]interface{})

		for _, content := range contents {
			vendorDirPath := filepath.Join(dirMap["path"].(string), content.(map[string]interface{})["path"].(string))
			contentMap := content.(map[string]interface{})
			cacheName, err := genCacheName(contentMap)
			if err != nil {
				return err
			}
			vendorDirToCacheMap[vendorDirPath] = cacheName
			cacheDir := a.expandVendirCache(cacheName)
			// FIXME: Possible race condition if multiple applications are running in parallel
			if err = v.saveCacheVendirConfig(a, cacheName, buildCacheVendirConfig(cacheDir, vendirConfig, dirMap, contentMap)); err != nil {
				return err
			}
		}
	}

	return v.saveLinksMap(a, vendorDirToCacheMap)
}

func (a *Application) getCacheVendirConfigPath(cacheName string) string {
	return path.Join(a.expandVendirCache(cacheName), a.e.g.VendirConfigFileName)
}

func (v *VendirSyncer) saveCacheVendirConfig(a *Application, cacheName string, vendirConfig map[string]interface{}) error {
	data, err := yaml.Marshal(vendirConfig)
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to marshal vendir config"))
		return err
	}
	vendirConfigPath := a.getCacheVendirConfigPath(cacheName)
	if err = writeFile(vendirConfigPath, data); err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to write vendir config"))
		return err
	}
	return nil
}

func buildCacheVendirConfig(cacheDir string, vendirConfig, vendirDirConfig, vendirContentConfig map[string]interface{}) map[string]interface{} {
	knownKeys := []string{"apiVersion", "kind", "minimumRequiredVersion"}
	newVendirConfig := map[string]interface{}{}
	for _, key := range knownKeys {
		if val, ok := vendirConfig[key]; ok {
			newVendirConfig[key] = val
		}
	}

	newDirConfig := map[string]interface{}{}
	// TODO: move "data" to the Globe config or to a constant
	newDirConfig["path"] = filepath.Join(cacheDir, "data")
	newDirConfig["permissions"] = vendirDirConfig["permissions"]

	vendirContentConfig["path"] = "."

	newDirConfig["contents"] = []interface{}{vendirContentConfig}
	newVendirConfig["directories"] = []interface{}{newDirConfig}
	return newVendirConfig
}

func (a *Application) getLinksMapPath() string {
	return a.expandServicePath(a.e.g.VendirLinksMapFileName)
}

func (v *VendirSyncer) saveLinksMap(a *Application, linksMap map[string]string) error {
	linksMapPath := a.getLinksMapPath()
	data, err := yaml.Marshal(linksMap)
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to marshal links map"))
		return err
	}
	if err = writeFile(linksMapPath, data); err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to write links map"))
		return err
	}
	return nil
}
