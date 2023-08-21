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

type Directory struct {
	Path        string
	ContentHash string `yaml:"contentHash"`
	Secret      string `yaml:"-"`
}

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
	if _, err := os.Stat(protoVendirDir); err == nil {
		yttFiles = append(yttFiles, protoVendirDir)
	}

	appVendirDirs := a.e.collectBySubpath(filepath.Join("_apps", a.Name, "vendir"))
	yttFiles = append(yttFiles, appVendirDirs...)

	if len(yttFiles) == 0 {
		err := ErrNoVendirConfig
		return err
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
	vendirConfigFilePath := filepath.Join(a.expandServicePath(""), a.e.g.VendirConfigFileName)
	vendirLockFilePath := filepath.Join(a.expandServicePath(""), a.e.g.VendirLockFileName)
	vendirSyncFilePath := a.expandTempPath(a.e.g.VendirSyncFileName)
	vendorDir := a.expandPath(a.e.g.VendorDirName)

	vendirDirs, err := readVendirConfig(vendirConfigFilePath)
	if err != nil {
		log.Error().Err(err).Msg(a.Msg(syncStepName, "Error while trying to find directories in vendir config: "+vendirConfigFilePath))
		return err
	}

	syncFileDirs, err := readSyncFile(vendirSyncFilePath)
	if err != nil {
		log.Error().Err(err).Msg(a.Msg(syncStepName, "Unable to read Vendir Sync file: "+vendirSyncFilePath))
		return err
	}

	lockFileDirs, err := readLockFile(vendirLockFilePath)
	if err != nil {
		log.Error().Err(err).Msg(a.Msg(syncStepName, "Unable to read Vendir Lock file: "+vendirLockFilePath))
		return err
	}

	err = createDirectory(vendorDir)
	if err != nil {
		log.Error().Err(err).Msg(a.Msg(syncStepName, "Unable to create vendor dir: "+vendorDir))
		return err
	}

	// TODO sync retry
	// only sync vendir with directory flag, if the lock file matches the vendir config file and caching is enabled
	if a.cached && checkLockFileMatch(vendirDirs, lockFileDirs) {
		for _, dir := range vendirDirs {
			if checkVersionMatch(dir.Path, dir.ContentHash, syncFileDirs) {
				log.Info().Msg(a.Msg(syncStepName, "Resource already synced"))
				continue
			}
			if err := a.runVendirSync(vendorDir, vendirConfigFileRelativePath, vendirLockFileRelativePath, vendirSecrets, dir.Path); err != nil {
				return err
			}
		}
	} else {
		if err := a.runVendirSync(vendorDir, vendirConfigFileRelativePath, vendirLockFileRelativePath, vendirSecrets, ""); err != nil {
			return err
		}
	}

	err = writeSyncFile(a.expandTempPath(a.e.g.VendirSyncFileName), vendirDirs)
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
	res, err := a.runCmd("vendir sync", "vendir", strings.NewReader(vendirSecrets), args)
	if err != nil {
		log.Error().Err(err).Str("stdout", res.Stdout).Str("stderr", res.Stderr).Msg(a.Msg(syncStepName, "Unable to sync vendir"))
		return err
	}
	log.Info().Msg(a.Msg(syncStepName, "Synced"))
	return nil
}

func writeSyncFile(syncFilePath string, directories []Directory) error {
	bytes, err := yaml.Marshal(directories)
	if err != nil {
		return err
	}
	err = writeFile(syncFilePath, bytes)
	if err != nil {
		return err
	}

	return nil
}

func readVendirConfig(vendirConfigFilePath string) ([]Directory, error) {
	config, err := unmarshalYamlToMap(vendirConfigFilePath)
	if err != nil {
		return nil, err
	}

	vendirDirs, err := findDirectories(config)
	if err != nil {
		return nil, err
	}

	return vendirDirs, nil
}

func readSyncFile(vendirSyncFile string) ([]Directory, error) {
	if _, err := os.Stat(vendirSyncFile); err != nil {
		return []Directory{}, nil
	}

	syncFile, err := os.ReadFile(vendirSyncFile)
	if err != nil {
		return nil, err
	}

	out := &[]Directory{}
	err = yaml.Unmarshal(syncFile, out)
	if err != nil {
		return nil, err
	}

	return *out, nil
}

func readLockFile(vendirLockFile string) ([]Directory, error) {
	config, err := unmarshalYamlToMap(vendirLockFile)
	if err != nil {
		return nil, err
	}

	if len(config) == 0 {
		return []Directory{}, nil
	}

	directories, err := findDirectories(config)
	if err != nil {
		return nil, err
	}

	return directories, nil
}

func findDirectories(config map[string]interface{}) ([]Directory, error) {
	// check if directories key exists
	if _, ok := config["directories"]; !ok {
		return nil, errors.New("no directories found in vendir config")
	}
	var syncDirs []Directory

	for _, dir := range config["directories"].([]interface{}) {
		dirMap := dir.(map[string]interface{})
		path := dirMap["path"].(string)
		// check contents length
		if len(dirMap["contents"].([]interface{})) > 1 {
			return nil, errors.New("Vendir config contains more than one contents for path: " + path + ". This is not supported")
		}
		contents := dirMap["contents"].([]interface{})[0].(map[string]interface{})
		subPath := contents["path"].(string)
		if subPath != "." {
			path += "/" + subPath
		}

		secret := ""
		if contents["imgpkgBundle"] != nil {
			imgpkgBundle := contents["imgpkgBundle"].(map[string]interface{})
			if imgpkgBundle["secretRef"] != nil {
				secretRef := imgpkgBundle["secretRef"].(map[string]interface{})
				secret = secretRef["name"].(string)
			}
		}

		if contents["helmChart"] != nil {
			helmChart := contents["helmChart"].(map[string]interface{})
			if helmChart["repository"] != nil {
				repository := helmChart["repository"].(map[string]interface{})
				if repository["secretRef"] != nil {
					secretRef := repository["secretRef"].(map[string]interface{})
					secret = secretRef["name"].(string)
				}
			}
		}

		sortedYaml, err := sortYaml(contents)
		if err != nil {
			return nil, err
		}
		syncDirs = append(syncDirs, Directory{
			Path:        path,
			ContentHash: hash(sortedYaml),
			Secret:      secret,
		})
	}
	return syncDirs, nil
}

func checkVersionMatch(path string, contentHash string, syncDirs []Directory) bool {
	for _, dir := range syncDirs {
		if dir.Path == path {
			if dir.ContentHash == contentHash {
				return true
			}
		}
	}
	return false
}

func checkPathMatch(path string, syncDirs []Directory) bool {
	for _, dir := range syncDirs {
		if dir.Path == path {
			return true
		}
	}
	return false
}

func checkLockFileMatch(vendirDirs []Directory, lockFileDirs []Directory) bool {
	for _, dir := range vendirDirs {
		if !checkPathMatch(dir.Path, lockFileDirs) {
			return false
		}
	}
	return true
}
