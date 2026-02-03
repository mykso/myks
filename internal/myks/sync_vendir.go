package myks

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	vendirconf "carvel.dev/vendir/pkg/vendir/config"
	"github.com/rs/zerolog/log"
	goyaml "gopkg.in/yaml.v3"
	"sigs.k8s.io/yaml"
)

const (
	VendirCacheDataDirName = "data"
)

var (
	// vendirCacheConfigMutex is used to prevent concurrent writes to the vendir cache config files in saveCacheVendirConfig function
	vendirCacheConfigMutex sync.Mutex

	// vendirSyncMutex is used to prevent concurrent vendir sync operations in runVendirSync function
	vendirSyncMutex sync.Mutex
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

	baseDir := filepath.Join(a.e.g.PrototypesDir, "_vendir")
	if ok, err := isExist(baseDir); err != nil {
		return err
	} else if ok {
		yttFiles = slices.Insert(yttFiles, 0, baseDir)
	}

	// add environment, prototype, and application data files
	yttFiles = slices.Insert(yttFiles, 0, a.yttDataFiles...)

	// create vendir config yaml
	vendirConfig, err := a.yttS(v.getStepName(), "creating vendir config", yttFiles, nil)
	if err != nil {
		return err
	}

	if vendirConfig.Stdout == "" {
		return errors.New("rendered empty vendir config")
	}

	if err := validateVendirConfig(vendirConfig.Stdout); err != nil {
		return fmt.Errorf("invalid vendir config: %w", err)
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
	linksMap, err := a.getLinksMap()
	if err != nil {
		return err
	}

	if err := os.RemoveAll(a.expandVendorPath("")); err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to remove vendor directory"))
		return err
	}

	for contentPath, cacheName := range linksMap {
		cacheDir := a.expandVendirCache(cacheName)
		vendirConfigPath := filepath.Join(cacheDir, a.e.g.VendirConfigFileName)
		vendirLockPath := filepath.Join(cacheDir, a.e.g.VendirLockFileName)
		if err := v.runVendirSync(a, vendirConfigPath, vendirLockPath, vendirSecrets); err != nil {
			log.Error().Err(err).Msg(a.Msg(v.getStepName(), "Vendir sync failed, cleaning up the cache entry"))
			if e := os.RemoveAll(cacheDir); e != nil {
				log.Warn().Err(e).Msg(a.Msg(v.getStepName(), "Unable to remove cache directory"))
			}
			return err
		}
		if err := v.linkVendorToCache(a, contentPath, cacheName); err != nil {
			log.Error().Err(err).Msg(a.Msg(v.getStepName(), "Unable to create link to cache"))
			return err
		}
	}

	return nil
}

func (v *VendirSyncer) linkVendorToCache(a *Application, vendorPath, cacheName string) error {
	linkFullPath := a.expandVendorPath(vendorPath)
	linkDir := filepath.Dir(linkFullPath)
	cacheDataPath := path.Join(a.expandVendirCache(cacheName), VendirCacheDataDirName)

	relCacheDataPath, err := filepath.Rel(linkDir, cacheDataPath)
	if err != nil {
		return err
	}

	if err := createDirectory(linkDir); err != nil {
		return err
	}

	return os.Symlink(relCacheDataPath, linkFullPath)
}

func (v *VendirSyncer) runVendirSync(a *Application, vendirConfig, vendirLock, vendirSecrets string) error {
	vendirSyncMutex.Lock()
	defer vendirSyncMutex.Unlock()
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
	configPath := a.expandServicePath(a.e.g.VendirConfigFileName)
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read vendir config: %w", err)
	}

	// Unmarshal directly without validation (NewConfigFromBytes validates which
	// may reject certain valid-for-our-use configs like those with "." paths)
	var vendirConfig vendirconf.Config
	if err := yaml.Unmarshal(configBytes, &vendirConfig); err != nil {
		return fmt.Errorf("failed to parse vendir config: %w", err)
	}

	vendorDirToCacheMap := map[string]string{}

	for _, dir := range vendirConfig.Directories {
		for i := range dir.Contents {
			content := &dir.Contents[i]
			vendorDirPath := filepath.Join(dir.Path, content.Path)
			cacheName, err := genCacheName(*content)
			if err != nil {
				return err
			}
			vendorDirToCacheMap[vendorDirPath] = cacheName
			cacheDir := a.expandVendirCache(cacheName)
			cacheConfig := buildCacheVendirConfig(cacheDir, vendirConfig, dir, content)
			if err = v.saveCacheVendirConfig(a, cacheName, cacheConfig); err != nil {
				return err
			}
		}
	}

	return v.saveLinksMap(a, vendorDirToCacheMap)
}

func (a *Application) getCacheVendirConfigPath(cacheName string) string {
	return path.Join(a.expandVendirCache(cacheName), a.e.g.VendirConfigFileName)
}

func (v *VendirSyncer) saveCacheVendirConfig(a *Application, cacheName string, vendirConfig vendirconf.Config) error {
	data, err := vendirConfig.AsBytes()
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to marshal vendir config"))
		return err
	}
	vendirConfigPath := a.getCacheVendirConfigPath(cacheName)
	vendirCacheConfigMutex.Lock()
	defer vendirCacheConfigMutex.Unlock()
	if err = writeFile(vendirConfigPath, data); err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to write vendir config"))
		return err
	}
	return nil
}

func buildCacheVendirConfig(cacheDir string, vendirConfig vendirconf.Config, vendirDir vendirconf.Directory, vendirContent *vendirconf.DirectoryContents) vendirconf.Config {
	// Create a copy of the content with path set to "."
	contentCopy := *vendirContent
	contentCopy.Path = "."

	// Ensure required fields have default values
	apiVersion := vendirConfig.APIVersion
	if apiVersion == "" {
		apiVersion = vendirAPIVersion
	}
	kind := vendirConfig.Kind
	if kind == "" {
		kind = "Config"
	}

	newConfig := vendirconf.Config{
		APIVersion:             apiVersion,
		Kind:                   kind,
		MinimumRequiredVersion: vendirConfig.MinimumRequiredVersion,
		Directories: []vendirconf.Directory{
			{
				Path:        filepath.Join(cacheDir, VendirCacheDataDirName),
				Permissions: vendirDir.Permissions,
				Contents:    []vendirconf.DirectoryContents{contentCopy},
			},
		},
	}
	return newConfig
}

const vendirAPIVersion = "vendir.k14s.io/v1alpha1"

// validateVendirConfig validates the rendered vendir configuration YAML.
// It checks for:
// - Single YAML document (no multiple documents separated by ---)
// - Required apiVersion field with correct value
// - Required kind field with correct value
// - At least one directory defined
// - Each directory has a path and at least one content entry
func validateVendirConfig(configYaml string) error {
	// Check for multiple YAML documents
	// A document separator is "\n---" which indicates the start of a new document after content
	// We don't allow any document separators (only one document allowed)
	if strings.Contains(configYaml, "\n---") {
		return errors.New("vendir config contains multiple YAML documents, expected exactly 1")
	}

	// Parse and validate structure
	var config vendirconf.Config
	if err := yaml.Unmarshal([]byte(configYaml), &config); err != nil {
		return fmt.Errorf("failed to parse vendir config: %w", err)
	}

	// Validate apiVersion
	if config.APIVersion == "" {
		return errors.New("vendir config missing required field: apiVersion")
	}
	if config.APIVersion != vendirAPIVersion {
		return fmt.Errorf("vendir config has invalid apiVersion: %q, expected %q", config.APIVersion, vendirAPIVersion)
	}

	// Validate kind
	if config.Kind == "" {
		return errors.New("vendir config missing required field: kind")
	}
	if config.Kind != "Config" {
		return fmt.Errorf("vendir config has invalid kind: %q, expected %q", config.Kind, "Config")
	}

	// Validate directories
	if len(config.Directories) == 0 {
		return errors.New("vendir config has no directories defined")
	}

	for i, dir := range config.Directories {
		if dir.Path == "" {
			return fmt.Errorf("vendir config directory[%d] missing required field: path", i)
		}
		if len(dir.Contents) == 0 {
			return fmt.Errorf("vendir config directory[%d] (%s) has no contents defined", i, dir.Path)
		}
	}

	return nil
}

func (a *Application) getLinksMap() (map[string]string, error) {
	linksMap := map[string]string{}
	linksMapRaw, err := unmarshalYamlToMap(a.getLinksMapPath())
	if err != nil {
		return nil, err
	}
	for k, v := range linksMapRaw {
		linksMap[k] = v.(string)
	}
	return linksMap, nil
}

func (a *Application) getLinksMapPath() string {
	return a.expandServicePath(a.e.g.VendirLinksMapFileName)
}

func (v *VendirSyncer) saveLinksMap(a *Application, linksMap map[string]string) error {
	linksMapPath := a.getLinksMapPath()
	data, err := goyaml.Marshal(linksMap)
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
