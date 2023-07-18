package myks

import (
	"errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

type Directory struct {
	Path        string
	ContentHash string `yaml:"contentHash"`
}

func (a *Application) doSync() error {
	// TODO: implement secrets-from-env extraction

	// Paths are relative to the vendor directory (BUG: this will brake with multi-level vendor directory, e.g. `vendor/shmendor`)
	vendirConfigFileRelativePath := filepath.Join("..", a.e.g.ServiceDirName, a.e.g.VendirConfigFileName)
	vendirLockFileRelativePath := filepath.Join("..", a.e.g.ServiceDirName, a.e.g.VendirLockFileName)
	vendirConfigFilePath := filepath.Join(a.expandServicePath(""), a.e.g.VendirConfigFileName)
	vendirLockFilePath := filepath.Join(a.expandServicePath(""), a.e.g.VendirLockFileName)
	vendirSyncPath := a.expandTempPath(a.e.g.VendirSyncFileName)
	vendorDir := a.expandPath(a.e.g.VendorDirName)

	vendirDirs, err := readVendirConfig(vendirConfigFilePath)
	if err != nil {
		log.Error().Err(err).Str("app", a.Name).Msg("Error while trying to find directories in vendir config: " + vendirConfigFilePath)
		return err
	}

	syncFileDirs, err := readSyncFile(vendirSyncPath)
	if err != nil {
		log.Error().Err(err).Str("app", a.Name).Msg("Unable to read Vendir Sync file: " + vendirSyncPath)
		return err
	}
	if len(syncFileDirs) == 0 {
		log.Debug().Str("app", a.Name).Msg("Vendir sync file not found. First sync..")
	}

	lockFileDirs, err := readLockFile(vendirLockFilePath)
	if err != nil {
		log.Error().Err(err).Str("app", a.Name).Msg("Unable to read Vendir Lock file: " + vendirLockFilePath)
		return err
	}

	err = createDirectory(vendorDir)
	if err != nil {
		log.Error().Err(err).Str("app", a.Name).Msg("Unable to create vendor dir: " + vendorDir)
		return err
	}

	//TODO sync retry
	// only sync vendir with directory flag, if the lock file matches the vendir config file and caching is enabled
	if a.cached && checkLockFileMatch(vendirDirs, lockFileDirs) {
		for _, dir := range vendirDirs {
			if checkVersionMatch(dir.Path, dir.ContentHash, syncFileDirs) {
				log.Debug().Str("app", a.Name).Msg("Skipping vendir sync for: " + dir.Path)
				continue
			}
			log.Info().Str("app", a.Name).Msg("Syncing vendir for: " + dir.Path)
			res, err := runCmd("vendir", nil, []string{
				"sync",
				"--chdir=" + vendorDir,
				"--directory=" + dir.Path,
				"--file=" + vendirConfigFileRelativePath,
				"--lock-file=" + vendirLockFileRelativePath,
			})
			if err != nil {
				log.Warn().Err(err).Str("app", a.Name).Str("stdout", res.Stdout).Str("stderr", res.Stderr).Msg("Unable to sync vendir")
				return err
			}
		}
	} else {
		log.Info().Str("app", a.Name).Msg("Syncing vendir completely for: " + vendirConfigFilePath)
		res, err := runCmd("vendir", nil, []string{
			"sync",
			"--chdir=" + vendorDir,
			"--file=" + vendirConfigFileRelativePath,
			"--lock-file=" + vendirLockFileRelativePath,
		})
		if err != nil {
			log.Error().Err(err).Str("app", a.Name).Str("stdout", res.Stdout).Str("stderr", res.Stderr).Msg("Unable to sync vendir")
			return err
		}
	}

	err = a.writeSyncFile(vendirDirs)
	if err != nil {
		log.Error().Str("app", a.Name).Err(err).Msg("Unable to write sync file")
		return err
	}

	log.Debug().Str("app", a.Name).Msg("Vendir sync file written: " + vendirSyncPath)
	log.Info().Str("app", a.Name).Msg("Vendir sync completed!")

	return nil
}

func (a *Application) writeSyncFile(directories []Directory) error {

	bytes, err := yaml.Marshal(directories)
	if err != nil {
		return err
	}
	err = a.writeTempFile(a.e.g.VendirSyncFileName, string(bytes))
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
	var directories = make(map[string]string)
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
		sortedYaml, err := sortYaml(contents)
		if err != nil {
			return nil, err
		}
		directories[path] = sortedYaml
	}
	return convertDirectoryMapToHashedStruct(directories), nil
}

func convertDirectoryMapToHashedStruct(directories map[string]string) []Directory {
	var syncDirs []Directory
	for path, contents := range directories {
		syncDirs = append(syncDirs, Directory{
			Path:        path,
			ContentHash: hash(contents),
		})
	}
	return syncDirs
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
