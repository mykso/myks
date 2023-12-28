package myks

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

const vendorDirOverlayTemplate = `
#@ load("@ytt:overlay", "overlay")
#@overlay/match by=overlay.subset({"kind": "Config", "apiVersion": "vendir.k14s.io/v1alpha1"})
---
directories:
  - #@overlay/match by=overlay.all, expects="1+"
    #@overlay/replace via=lambda l, r: (r + "/" + l).replace("//", "/")
    path: %s`

func (a *Application) Sync(vendirSecrets string) error {
	log.Debug().Msg(a.Msg(syncStepName, "Starting"))
	if err := a.prepareSync(); err != nil {
		if err == ErrNoVendirConfig {
			log.Info().Msg(a.Msg(syncStepName, "No vendir config found"))
			return nil
		}
		return err
	}

	if err := a.doSync(vendirSecrets); err != nil {
		return err
	}

	return nil
}

func (a *Application) prepareSync() error {
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

	vendorDir := a.expandPath(a.e.g.VendorDirName)
	overlayReader := strings.NewReader(fmt.Sprintf(vendorDirOverlayTemplate, vendorDir))
	vendirConfig, err := a.yttS(syncStepName, "creating vendir config", yttFiles, overlayReader)
	if err != nil {
		return err
	}

	if vendirConfig.Stdout == "" {
		err = errors.New("Empty vendir config")
		return err
	}

	vendirConfigFilePath := a.expandServicePath(a.e.g.VendirConfigFileName)
	// Create directory if it does not exist
	err = os.MkdirAll(filepath.Dir(vendirConfigFilePath), 0o750)
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(syncStepName, "Unable to create directory for vendir config file"))
		return err
	}
	err = os.WriteFile(vendirConfigFilePath, []byte(vendirConfig.Stdout), 0o600)
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(syncStepName, "Unable to write vendir config file"))
		return err
	}

	return nil
}

func (a *Application) doSync(vendirSecrets string) error {
	vendirConfigPath := a.expandServicePath(a.e.g.VendirConfigFileName)
	vendirLockFilePath := a.expandServicePath(a.e.g.VendirLockFileName)

	// TODO sync retry
	if err := a.runVendirSync(vendirConfigPath, vendirLockFilePath, vendirSecrets); err != nil {
		log.Error().Err(err).Msg(a.Msg(syncStepName, "Vendir sync failed"))
		return err
	}

	vendorDir := a.expandPath(a.e.g.VendorDirName)
	return a.cleanupVendorDir(vendorDir, vendirConfigPath)
}

func (a *Application) runVendirSync(vendirConfig, vendirLock, vendirSecrets string) error {
	args := []string{
		"sync",
		"--file=" + vendirConfig,
		"--lock-file=" + vendirLock,
		"--file=-",
	}
	_, err := a.runCmd(syncStepName, "vendir sync", "vendir", strings.NewReader(vendirSecrets), args)
	if err != nil {
		return err
	}
	log.Info().Msg(a.Msg(syncStepName, "Synced"))
	return nil
}

func (a Application) cleanupVendorDir(vendorDir, vendirConfigFile string) error {
	config, err := unmarshalYamlToMap(vendirConfigFile)
	if err != nil {
		return err
	}

	if _, ok := config["directories"]; !ok {
		return errors.New("no directories found in vendir config")
	}

	dirs := []string{}
	for _, dir := range config["directories"].([]interface{}) {
		dirMap := dir.(map[string]interface{})
		path := dirMap["path"].(string)
		dirs = append(dirs, filepath.Clean(path)+string(filepath.Separator))
	}

	log.Debug().Strs("vendir-managed dirs", dirs).Msg("")

	return filepath.WalkDir(vendorDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		path = path + string(filepath.Separator)
		for _, dir := range dirs {
			if dir == path {
				return fs.SkipDir
			}

			if strings.HasPrefix(dir, path) {
				return nil
			}

			// This should never happen
			if strings.HasPrefix(path, dir) {
				log.Debug().Msgf("%s has prefix %s", path, dir)
				return fs.SkipDir
			}
		}
		log.Debug().Msg(a.Msg(syncStepName, "Removing directory "+path))
		os.RemoveAll(path)

		return fs.SkipDir
	})
}
