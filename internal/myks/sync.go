package myks

import (
	_ "embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

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

	vendirConfig, err := a.ytt(syncStepName, "creating vendir config", yttFiles)
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
	// Paths are relative to the vendor directory (BUG: this will brake with multi-level vendor directory, e.g. `vendor/shmendor`)
	vendirConfigFileRelativePath := filepath.Join("..", a.e.g.ServiceDirName, a.e.g.VendirConfigFileName)
	vendirLockFileRelativePath := filepath.Join("..", a.e.g.ServiceDirName, a.e.g.VendirLockFileName)
	vendorDir := a.expandPath(a.e.g.VendorDirName)
	// remove old content of vendor directory, since there might be leftovers in case of path changes
	if exists, _ := isExist(vendorDir); exists {
		if err := os.RemoveAll(vendorDir); err != nil {
			return err
		}
	}
	if err := createDirectory(vendorDir); err != nil {
		log.Error().Err(err).Msg(a.Msg(syncStepName, "Unable to create vendor dir: "+vendorDir))
		return err
	}
	// TODO sync retry
	if err := a.runVendirSync(vendorDir, vendirConfigFileRelativePath, vendirLockFileRelativePath, vendirSecrets); err != nil {
		log.Error().Err(err).Msg(a.Msg(syncStepName, "Vendir sync failed"))
		return err
	}

	vendirConfigFile := a.expandServicePath(a.e.g.VendirConfigFileName)
	return a.cleanupVendorDir(vendorDir, vendirConfigFile)
}

func (a *Application) runVendirSync(targetDir string, vendirConfig string, vendirLock string, vendirSecrets string) error {
	args := []string{
		"sync",
		"--chdir=" + targetDir,
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
		dirs = append(dirs, path+string(filepath.Separator))
	}

	log.Debug().Strs("managed dirs", dirs).Msg("")

	return filepath.WalkDir(vendorDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}
		log.Debug().Msg(a.Msg(syncStepName, "Checking directory "+path))

		relPath, err := filepath.Rel(vendorDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		relPath = relPath + string(filepath.Separator)
		for _, dir := range dirs {
			log.Debug().Str("dir", dir).Str("relPath", relPath).Msg("Checking dir")
			if dir == relPath {
				log.Debug().Msgf("%s == %s", dir, relPath)
				return fs.SkipDir
			}

			if strings.HasPrefix(dir, relPath) {
				log.Debug().Msgf("%s has prefix %s", dir, relPath)
				return nil
			}

			// This should never happen
			if strings.HasPrefix(relPath, dir) {
				log.Debug().Msgf("%s has prefix %s", relPath, dir)
				return fs.SkipDir
			}
		}
		log.Debug().Msg(a.Msg(syncStepName, "Removing directory "+path))
		os.RemoveAll(path)

		return fs.SkipDir
	})
}
