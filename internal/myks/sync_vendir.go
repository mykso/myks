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

	if err := v.patchVendirConfig(a); err != nil {
		return err
	}

	return nil
}

func (v *VendirSyncer) doSync(a *Application, vendirSecrets string) error {
	vendirPatchedConfigPath := a.expandServicePath(a.e.g.VendirPatchedConfigFileName)
	vendirLockFilePath := a.expandServicePath(a.e.g.VendirLockFileName)

	vendirPatchedConfig, err := unmarshalYamlToMap(vendirPatchedConfigPath)
	if err != nil {
		return err
	}

	for _, dir := range vendirPatchedConfig["directories"].([]interface{}) {
		dirMap := dir.(map[string]interface{})
		dirPath := dirMap["path"].(string)
		for _, content := range dirMap["contents"].([]interface{}) {
			contentMap := content.(map[string]interface{})
			contentPath := contentMap["path"].(string)
			syncPath := filepath.Join(dirPath, contentPath)
			if err := v.runVendirSync(a, vendirPatchedConfigPath, syncPath, vendirLockFilePath, vendirSecrets); err != nil {
				log.Error().Err(err).Msg(a.Msg(v.getStepName(), "Vendir sync failed"))
				err := removeDirectory(syncPath)
				return err
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

func (v *VendirSyncer) getCacheConfig(a *Application) (CacheConfig, error) {
	dataValuesYaml, err := a.ytt(v.getStepName(), "get cache config", a.yttDataFiles, "--data-values-inspect")
	if err != nil {
		return CacheConfig{}, err
	}

	var cacheConfig struct {
		Cache CacheConfig
	}
	err = yaml.Unmarshal([]byte(dataValuesYaml.Stdout), &cacheConfig)
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to unmarshal data values"))
		return CacheConfig{}, err
	}

	return cacheConfig.Cache, nil
}

func (v *VendirSyncer) patchVendirConfig(a *Application) error {
	vendirConfig, err := unmarshalYamlToMap(a.expandServicePath(a.e.g.VendirConfigFileName))
	if err != nil {
		return err
	}

	vendirPatchedConfigPath := a.expandServicePath(a.e.g.VendirPatchedConfigFileName)
	cacheConfig, err := v.getCacheConfig(a)
	if err != nil {
		return err
	}

	for _, dir := range vendirConfig["directories"].([]interface{}) {
		dirMap := dir.(map[string]interface{})
		dirPath := dirMap["path"].(string)
		contents := dirMap["contents"].([]interface{})
		if len(contents) == 0 {
			log.Warn().Str("directory", dirPath).Msg(a.Msg(v.getStepName(), "No contents found in vendir config directory"))
			continue
		} else if len(contents) > 1 {
			log.Warn().Str("directory", dirPath).Msg(a.Msg(v.getStepName(), "Multiple contents found in vendir config directory"))
			return errors.New("multiple contents are not supported in vendir config directory")
		}
		contentMap := contents[0].(map[string]interface{})
		contentPath := contentMap["path"].(string)

		path := filepath.Join(dirPath, contentPath)
		cacheName, err := findCacheNamer(contentMap).Name(path, contentMap)
		if err != nil {
			return err
		}
		outputPath := filepath.Join(a.expandVendirCache(cacheName), dirPath)
		dirMap["path"] = outputPath
		exists, _ := isExist(outputPath)
		if _, ok := contentMap["lazy"]; !ok && cacheConfig.Enabled && exists {
			contentMap["lazy"] = true
		}
	}

	data, err := yaml.Marshal(vendirConfig)
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to marshal patched vendir config"))
		return err
	}

	if err = writeFile(vendirPatchedConfigPath, data); err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to write patched vendir config"))
		return err
	}

	return nil
}
