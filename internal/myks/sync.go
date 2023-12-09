package myks

import (
	_ "embed"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

type vendirDirHashes map[string]string

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
		log.Warn().Err(err).Msg(a.Msg(syncStepName, "Unable to render vendir config"))
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
	vendirConfigFilePath := a.expandServicePath(a.e.g.VendirConfigFileName)
	vendirLockFilePath := a.expandServicePath(a.e.g.VendirLockFileName)
	vendirSyncFilePath := a.expandServicePath(a.e.g.VendirSyncFileName)
	vendorDir := a.expandPath(a.e.g.VendorDirName)

	vendirDirHashes, err := readVendirDirHashes(vendirConfigFilePath)
	if err != nil {
		log.Error().Err(err).Msg(a.Msg(syncStepName, "Error while trying to find directories in vendir config: "+vendirConfigFilePath))
		return err
	}

	syncFileDirHashes, err := readSyncFile(vendirSyncFilePath)
	if err != nil {
		log.Error().Err(err).Msg(a.Msg(syncStepName, "Unable to read Vendir Sync file: "+vendirSyncFilePath))
		return err
	}

	// having hashes here is actually not necessary, since we only need the paths, but it's easier to just reuse the function
	lockFileDirHashes, err := readLockFileDirHashes(vendirLockFilePath)
	if err != nil {
		log.Error().Err(err).Msg(a.Msg(syncStepName, "Unable to read Vendir Lock file: "+vendirLockFilePath))
		return err
	}

	exist, err := isExist(vendorDir)
	if err != nil {
		log.Error().Err(err).Msg(a.Msg(syncStepName, "Unable to check if vendor dir exists"))
		return err
	}

	// TODO sync retry
	// only sync vendir with directory flag, if the lock file matches the vendir config file and caching is enabled
	if exist && a.useCache && checkLockFileMatch(vendirDirHashes, lockFileDirHashes) {
		for dir, hash := range vendirDirHashes {
			if checkVersionMatch(dir, hash, syncFileDirHashes) {
				log.Info().Str("vendir dir", dir).Msg(a.Msg(syncStepName, "Resource already synced"))
				continue
			}
			if err := a.runVendirSync(vendorDir, vendirConfigFileRelativePath, vendirLockFileRelativePath, vendirSecrets, dir); err != nil {
				log.Error().Err(err).Msg(a.Msg(syncStepName, "Vendir sync failed"))
				return err
			}
		}
	} else {
		// remove old content of vendor directory, since there might be leftovers in case of path changes
		if err := os.RemoveAll(vendorDir); err != nil {
			return err
		}

		if err := createDirectory(vendorDir); err != nil {
			log.Error().Err(err).Msg(a.Msg(syncStepName, "Unable to create vendor dir: "+vendorDir))
			return err
		}

		if err := a.runVendirSync(vendorDir, vendirConfigFileRelativePath, vendirLockFileRelativePath, vendirSecrets, ""); err != nil {
			log.Error().Err(err).Msg(a.Msg(syncStepName, "Vendir sync failed"))
			return err
		}
	}

	err = writeSyncFile(a.expandServicePath(a.e.g.VendirSyncFileName), vendirDirHashes)
	if err != nil {
		log.Error().Err(err).Msg(a.Msg(syncStepName, "Unable to write sync file"))
		return err
	}

	return nil
}

func (a *Application) runVendirSync(targetDir string, vendirConfig string, vendirLock string, vendirSecrets string, directory string) error {
	args := []string{
		"sync",
		"--chdir=" + targetDir,
		"--file=" + vendirConfig,
		"--lock-file=" + vendirLock,
		"--file=-",
	}
	if directory != "" {
		args = append(args, "--directory="+directory)
	}
	res, err := a.runCmd(syncStepName, "vendir sync", "vendir", strings.NewReader(vendirSecrets), args)
	if err != nil {
		log.Error().Err(err).Str("stdout", res.Stdout).Str("stderr", res.Stderr).Msg(a.Msg(syncStepName, "Unable to sync vendir"))
		return err
	}
	log.Info().Msg(a.Msg(syncStepName, "Synced"))
	return nil
}

func writeSyncFile(syncFilePath string, dirHashes vendirDirHashes) error {
	bytes, err := yaml.Marshal(dirHashes)
	if err != nil {
		return err
	}
	err = writeFile(syncFilePath, bytes)
	if err != nil {
		return err
	}

	return nil
}

func readVendirDirHashes(vendirConfigFilePath string) (vendirDirHashes, error) {
	config, err := unmarshalYamlToMap(vendirConfigFilePath)
	if err != nil {
		return nil, err
	}

	return getVendirDirHashes(config)
}

func readSyncFile(vendirSyncFile string) (vendirDirHashes, error) {
	if ok, err := isExist(vendirSyncFile); err != nil {
		return nil, err
	} else if !ok {
		return vendirDirHashes{}, nil
	}

	syncFile, err := os.ReadFile(vendirSyncFile)
	if err != nil {
		return nil, err
	}

	out := &vendirDirHashes{}
	err = yaml.Unmarshal(syncFile, out)

	return *out, err
}

func readLockFileDirHashes(vendirLockFile string) (vendirDirHashes, error) {
	config, err := unmarshalYamlToMap(vendirLockFile)
	if err != nil {
		return nil, err
	}

	if len(config) == 0 {
		return vendirDirHashes{}, nil
	}

	return getVendirDirHashes(config)
}

func getVendirDirHashes(config map[string]interface{}) (vendirDirHashes, error) {
	// check if directories key exists
	if _, ok := config["directories"]; !ok {
		return nil, errors.New("no directories found in vendir config")
	}

	dirHashes := vendirDirHashes{}

	for _, dir := range config["directories"].([]interface{}) {
		dirMap := dir.(map[string]interface{})
		path := dirMap["path"].(string)
		contents := dirMap["contents"].([]interface{})
		for _, content := range contents {
			contentMap := content.(map[string]interface{})
			contentPath := contentMap["path"].(string)
			sortedYaml, err := sortYaml(contentMap)
			if err != nil {
				return nil, err
			}

			dirHashes[filepath.Join(path, contentPath)] = hashString(sortedYaml)
		}
	}
	return dirHashes, nil
}

func checkVersionMatch(dir, desiredHash string, hashedDirs vendirDirHashes) bool {
	if hash, ok := hashedDirs[dir]; ok {
		return hash == desiredHash
	}
	return false
}

func checkLockFileMatch(vendirDirHashes vendirDirHashes, lockFileDirHashes vendirDirHashes) bool {
	if len(vendirDirHashes) != len(lockFileDirHashes) {
		return false
	}
	for dir := range vendirDirHashes {
		if _, ok := lockFileDirHashes[dir]; !ok {
			return false
		}
	}
	return true
}
