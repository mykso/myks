package myks

import (
	"errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

const envPrefix = "VENDIR_SECRET_"

type Directory struct {
	Path        string
	ContentHash string `yaml:"contentHash"`
	Secret      string `yaml:"-"`
}

func (a *Application) Sync() error {
	if err := a.prepareSync(); err != nil {
		if err == ErrNoVendirConfig {
			return nil
		}
		return err
	}

	if err := a.doSync(); err != nil {
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
		log.Debug().Str("dir", protoVendirDir).Msg("Using prototype vendir directory")
	}

	appVendirDirs := a.e.collectBySubpath(filepath.Join("_apps", a.Name, "vendir"))
	yttFiles = append(yttFiles, appVendirDirs...)

	if len(yttFiles) == 0 {
		err := ErrNoVendirConfig
		log.Warn().Err(err).Str("app", a.Name).Msg("")
		return err
	}

	vendirConfig, err := a.e.g.ytt(yttFiles)
	if err != nil {
		log.Warn().Err(err).Str("app", a.Name).Msg("Unable to render vendir config")
		return err
	}

	if vendirConfig.Stdout == "" {
		err = errors.New("Empty vendir config")
		log.Warn().Err(err).Msg("")
		return err
	}

	vendirConfigFilePath := a.expandServicePath(a.e.g.VendirConfigFileName)
	// Create directory if it does not exist
	err = os.MkdirAll(filepath.Dir(vendirConfigFilePath), 0o750)
	if err != nil {
		log.Warn().Err(err).Msg("Unable to create directory for vendir config file")
		return err
	}
	err = os.WriteFile(vendirConfigFilePath, []byte(vendirConfig.Stdout), 0o600)
	if err != nil {
		log.Warn().Err(err).Msg("Unable to write vendir config file")
		return err
	}
	log.Debug().Str("app", a.Name).Str("file", vendirConfigFilePath).Msg("Wrote vendir config file")

	return nil
}

func (a *Application) doSync() error {
	// Paths are relative to the vendor directory (BUG: this will brake with multi-level vendor directory, e.g. `vendor/shmendor`)
	vendirConfigFileRelativePath := filepath.Join("..", a.e.g.ServiceDirName, a.e.g.VendirConfigFileName)
	vendirLockFileRelativePath := filepath.Join("..", a.e.g.ServiceDirName, a.e.g.VendirLockFileName)
	vendirConfigFilePath := filepath.Join(a.expandServicePath(""), a.e.g.VendirConfigFileName)
	vendirLockFilePath := filepath.Join(a.expandServicePath(""), a.e.g.VendirLockFileName)
	vendirSyncFilePath := a.expandTempPath(a.e.g.VendirSyncFileName)
	vendorDir := a.expandPath(a.e.g.VendorDirName)

	vendirDirs, err := readVendirConfig(vendirConfigFilePath)
	if err != nil {
		log.Error().Err(err).Str("app", a.Name).Msg("Error while trying to find directories in vendir config: " + vendirConfigFilePath)
		return err
	}

	syncFileDirs, err := readSyncFile(vendirSyncFilePath)
	if err != nil {
		log.Error().Err(err).Str("app", a.Name).Msg("Unable to read Vendir Sync file: " + vendirSyncFilePath)
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

	var secretFilePaths []string
	//TODO sync retry
	// only sync vendir with directory flag, if the lock file matches the vendir config file and caching is enabled
	if a.cached && checkLockFileMatch(vendirDirs, lockFileDirs) {
		for _, dir := range vendirDirs {
			if checkVersionMatch(dir.Path, dir.ContentHash, syncFileDirs) {
				log.Debug().Str("app", a.Name).Msg("Skipping vendir sync for: " + dir.Path)
				continue
			}
			log.Info().Str("app", a.Name).Msg("Syncing vendir for: " + dir.Path)
			args := []string{
				"sync",
				"--chdir=" + vendorDir,
				"--directory=" + dir.Path,
				"--file=" + vendirConfigFileRelativePath,
				"--lock-file=" + vendirLockFileRelativePath,
			}
			args, secretFilePath, err := handleVendirSecret(a.Name, dir, a.expandTempPath(""), filepath.Join("..", a.e.g.ServiceDirName, a.e.g.TempDirName), args)
			if err != nil {
				log.Error().Err(err).Str("app", a.Name).Msg("Unable to create secret for: " + dir.Path)
				return err
			}
			if secretFilePath != "" {
				secretFilePaths, _ = appendIfNotExists(secretFilePaths, secretFilePath)
			}

			res, err := runCmd("vendir", nil, args)
			if err != nil {
				log.Warn().Err(err).Str("app", a.Name).Str("stdout", res.Stdout).Str("stderr", res.Stderr).Msg("Unable to sync vendir")
				return err
			}
		}

	} else {
		log.Info().Str("app", a.Name).Msg("Syncing vendir completely for: " + vendirConfigFilePath)
		args := []string{
			"sync",
			"--chdir=" + vendorDir,
			"--file=" + vendirConfigFileRelativePath,
			"--lock-file=" + vendirLockFileRelativePath,
		}
		for _, dir := range vendirDirs {
			var secretFilePath string
			args, secretFilePath, err = handleVendirSecret(a.Name, dir, a.expandTempPath(""), filepath.Join("..", a.e.g.ServiceDirName, a.e.g.TempDirName), args)
			if err != nil {
				log.Error().Err(err).Str("app", a.Name).Msg("Unable to create secret for: " + dir.Path)
				return err
			}
			if secretFilePath != "" {
				secretFilePaths, _ = appendIfNotExists(secretFilePaths, secretFilePath)
			}

		}
		res, err := runCmd("vendir", nil, args)
		if err != nil {
			log.Error().Err(err).Str("app", a.Name).Str("stdout", res.Stdout).Str("stderr", res.Stderr).Msg("Unable to sync vendir")
			return err
		}
	}

	// make sure secrets do not linger on disk
	for _, secretFilePath := range secretFilePaths {
		defer func(name string) {
			log.Debug().Str("app", a.Name).Msg("delete secret file: " + name)
			err := os.Remove(name)
			if err != nil {
				log.Error().Err(err).Str("app", a.Name).Msg("unable to delete secret file: " + name)
			}
		}(secretFilePath)
	}

	err = writeSyncFile(a.expandTempPath(a.e.g.VendirSyncFileName), vendirDirs)
	if err != nil {
		log.Error().Str("app", a.Name).Err(err).Msg("Unable to write sync file")
		return err
	}

	log.Debug().Str("app", a.Name).Msg("Vendir sync file written: " + vendirSyncFilePath)
	log.Info().Str("app", a.Name).Msg("Vendir sync completed!")

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

func handleVendirSecret(app string, dir Directory, tempPath string, tempRelativePath string, vendirArgs []string) ([]string, string, error) {
	if dir.Secret != "" {
		username, password, err := getEnvCreds(dir.Secret)
		if err != nil {
			return vendirArgs, "", err
		}
		secretFileName := dir.Secret + ".yaml"
		secretFilePath := filepath.Join(tempPath, secretFileName)
		err = writeSecretFile(dir.Secret, secretFilePath, username, password)
		if err != nil {
			return vendirArgs, "", err
		}
		secretRelativePath := filepath.Join(tempRelativePath, secretFileName)
		var addedSecret bool
		vendirArgs, addedSecret = appendIfNotExists(vendirArgs, "--file="+secretRelativePath)
		if addedSecret {
			return vendirArgs, secretFilePath, nil
		} else {
			return vendirArgs, "", nil
		}
	}
	return vendirArgs, "", nil
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

func getEnvCreds(secretName string) (string, string, error) {
	username := os.Getenv(envPrefix + strings.ToUpper(secretName) + "_USERNAME")
	password := os.Getenv(envPrefix + strings.ToUpper(secretName) + "_PASSWORD")
	if username == "" || password == "" {
		return "", "", errors.New("no credentials found in environment for secret: " + secretName)
	}
	return username, password, nil
}
