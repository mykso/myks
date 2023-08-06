package myks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
)

type CmdResult struct {
	Stdout string
	Stderr string
}

func reductSecrets(args []string) []string {
	sensitiveFields := []string{"password", "secret", "token"}
	var logArgs []string
	for _, arg := range args {
		pattern := "(" + strings.Join(sensitiveFields, "|") + ")=(\\S+)"
		regex := regexp.MustCompile(pattern)
		logArgs = append(logArgs, regex.ReplaceAllString(arg, "$1=[REDACTED]"))
	}
	return logArgs
}

func processItemsInParallel(collection interface{}, fn func(interface{}) error) error {
	var items []interface{}

	value := reflect.ValueOf(collection)

	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array && value.Kind() != reflect.Map {
		return fmt.Errorf("collection must be a slice, array or map, got %s", value.Kind())
	}

	if value.Kind() == reflect.Map {
		for _, key := range value.MapKeys() {
			items = append(items, value.MapIndex(key).Interface())
		}
	} else {
		for i := 0; i < value.Len(); i++ {
			items = append(items, value.Index(i).Interface())
		}
	}

	errChan := make(chan error, len(items))
	defer close(errChan)

	for _, item := range items {
		go func(item interface{}) {
			errChan <- fn(item)
		}(item)
	}

	// Catch the first error
	var err error
	for range items {
		if subErr := <-errChan; subErr != nil && err == nil {
			err = fmt.Errorf("failed to process item: %w", subErr)
		}
	}

	return err
}

func copyFileSystemToPath(source fs.FS, sourcePath string, destinationPath string) error {
	if err := os.MkdirAll(destinationPath, 0o750); err != nil {
		return err
	}
	err := fs.WalkDir(source, sourcePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory
		if path == sourcePath {
			return nil
		}

		// Construct the corresponding destination path
		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			// This should never happen
			return err
		}
		destination := filepath.Join(destinationPath, relPath)

		log.Trace().
			Str("source", path).
			Str("destination", destination).
			Bool("isDir", d.IsDir()).
			Msg("Copying file")

		if d.IsDir() {
			// Create the destination directory
			if err := os.MkdirAll(destination, 0o750); err != nil {
				return err
			}
		} else {
			// Open the source file
			srcFile, err := source.Open(path)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			// Create the destination file
			dstFile, err := os.Create(destination)
			if err != nil {
				return err
			}
			defer dstFile.Close()

			// Copy the contents of the source file to the destination file
			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

func unmarshalYamlToMap(filePath string) (map[string]interface{}, error) {
	if _, err := os.Stat(filePath); err != nil {
		log.Debug().Str("filePath", filePath).Msg("Yaml not found.")
		return make(map[string]interface{}), nil
	}

	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func sortYaml(yaml map[string]interface{}) (string, error) {
	if yaml == nil {
		return "", nil
	}
	var sorted bytes.Buffer
	_, err := fmt.Fprint(&sorted, yaml)
	if err != nil {
		return "", err
	}
	return sorted.String(), nil
}

// hash string
func hash(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

func createDirectory(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		err := os.MkdirAll(dir, 0o750)
		if err != nil {
			log.Error().Err(err).Msg("Unable to create directory: " + dir)
			return err
		}
	}
	return nil
}

func writeFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); err != nil {
		err := os.MkdirAll(dir, 0o750)
		if err != nil {
			log.Warn().Err(err).Msg("Unable to create directory")
			return err
		}
	}

	return os.WriteFile(path, content, 0o600)
}

func appendIfNotExists(slice []string, element string) ([]string, bool) {
	for _, item := range slice {
		if item == element {
			return slice, false
		}
	}

	return append(slice, element), true
}

func getSubDirs(rootDir string) []string {
	if rootDir == "" {
		return []string{}
	}
	var resourceDirs []string
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && path != rootDir {
			resourceDirs = append(resourceDirs, path)
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		log.Warn().Err(err).Msg("Unable to walk vendor package directory")
		return []string{}
	}

	return resourceDirs
}

func runCmd(name string, stdin io.Reader, args []string, log func(name string, args []string)) (CmdResult, error) {
	cmd := exec.Command(name, args...)

	if stdin != nil {
		cmd.Stdin = stdin
	}

	var stdoutBs, stderrBs bytes.Buffer
	cmd.Stdout = &stdoutBs
	cmd.Stderr = &stderrBs

	err := cmd.Run()

	if log != nil {
		log(name, args)
	}

	return CmdResult{
		Stdout: stdoutBs.String(),
		Stderr: stderrBs.String(),
	}, err
}

func msgRunCmd(purpose string, cmd string, args []string) string {
	msg := cmd + " " + strings.Join(reductSecrets(args), " ")
	return "Running \u001B[34m" + cmd + "\u001B[0m to: \u001B[3m" + purpose + "\u001B[0m\n\u001B[37m" + msg + "\u001B[0m"
}

func runYttWithFilesAndStdin(paths []string, stdin io.Reader, log func(name string, args []string), args ...string) (CmdResult, error) {
	if stdin != nil {
		paths = append(paths, "-")
	}

	cmdArgs := []string{}
	for _, path := range paths {
		cmdArgs = append(cmdArgs, "--file="+path)
	}

	cmdArgs = append(cmdArgs, args...)
	res, err := runCmd("ytt", stdin, cmdArgs, log)
	if err != nil {
		return res, err
	}

	return res, err
}
