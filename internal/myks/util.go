package myks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	aurora "github.com/logrusorgru/aurora/v4"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"
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

func printFileNicely(name, content, syntax string) {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Println(content)
		return
	}

	fmt.Println(aurora.Bold(fmt.Sprintf("=== %s ===\n", name)))
	err := quick.Highlight(os.Stdout, content, syntax, "terminal16m", "doom-one2")
	if err != nil {
		log.Error().Err(err).Msg("Failed to highlight")
	} else {
		fmt.Printf("\n\n")
	}
}

func process(asyncLevel int, collection interface{}, fn func(interface{}) error) error {
	var items []interface{}

	value := reflect.ValueOf(collection)
	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			items = append(items, value.Index(i).Interface())
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			items = append(items, value.MapIndex(key).Interface())
		}
	default:
		return fmt.Errorf("collection must be a slice, array or map, got %s", value.Kind())
	}

	var eg errgroup.Group
	if asyncLevel == 0 { // no limit
		asyncLevel = -1
	}
	eg.SetLimit(asyncLevel)

	for _, item := range items {
		item := item // Create a new variable to avoid capturing the same item in the closure
		eg.Go(func() error {
			return fn(item)
		})
	}

	return eg.Wait()
}

func copyFileSystemToPath(source fs.FS, sourcePath string, destinationPath string) (err error) {
	if err = os.MkdirAll(destinationPath, 0o750); err != nil {
		return err
	}
	err = fs.WalkDir(source, sourcePath, func(path string, d fs.DirEntry, ferr error) error {
		if ferr != nil {
			return ferr
		}

		// Skip the root directory
		if path == sourcePath {
			return nil
		}

		// Construct the corresponding destination path
		relPath, ferr := filepath.Rel(sourcePath, path)
		if ferr != nil {
			// This should never happen
			return ferr
		}
		destination := filepath.Join(destinationPath, relPath)

		log.Trace().
			Str("source", path).
			Str("destination", destination).
			Bool("isDir", d.IsDir()).
			Msg("Copying file")

		if d.IsDir() {
			// Create the destination directory
			if ferr = os.MkdirAll(destination, 0o750); ferr != nil {
				return ferr
			}
		} else {

			// Open the source file
			srcFile, ferr := source.Open(path)
			if ferr != nil {
				return ferr
			}

			saveClose := func(srcFile fs.File) {
				closeErr := srcFile.Close()
				err = errors.Join(err, closeErr)
			}

			defer saveClose(srcFile)

			// Create the destination file
			dstFile, ferr := os.Create(destination)
			if ferr != nil {
				return ferr
			}
			defer saveClose(dstFile)

			// Copy the contents of the source file to the destination file
			_, ferr = io.Copy(dstFile, srcFile)
			if ferr != nil {
				return ferr
			}
		}

		return nil
	})

	return err
}

func unmarshalYamlToMap(filePath string) (map[string]interface{}, error) {
	ok, err := isExist(filePath)
	if err != nil {
		return nil, err
	}
	if !ok {
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

func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

func createDirectory(dir string) error {
	if ok, err := isExist(dir); err != nil {
		return err
	} else if ok {
		return nil
	}

	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Error().Err(err).Str("dir", dir).Msg("Unable to create directory")
		return err
	}
	return nil
}

func writeFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if ok, err := isExist(dir); err != nil {
		return err
	} else if !ok {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			log.Error().Err(err).Msg("Unable to create directory")
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

func getSubDirs(dir string) (subDirs []string, err error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			subDirs = append(subDirs, filepath.Join(dir, file.Name()))
		}
	}

	return
}

func runCmd(name string, stdin io.Reader, args []string, logFn func(name string, err error, stderr string, args []string)) (CmdResult, error) {
	cmd := exec.Command(name, args...)

	if stdin != nil {
		cmd.Stdin = stdin
	}

	var stdoutBs, stderrBs bytes.Buffer
	cmd.Stdout = &stdoutBs
	cmd.Stderr = &stderrBs

	err := cmd.Run()

	if logFn != nil {
		logFn(name, err, stderrBs.String(), args)
	}

	return CmdResult{
		Stdout: stdoutBs.String(),
		Stderr: stderrBs.String(),
	}, err
}

func msgRunCmd(purpose string, cmd string, args []string) string {
	msg := cmd + " " + strings.Join(reductSecrets(args), " ")
	if purpose == "" {
		return "Running \u001B[34m" + cmd + "\u001B[0m\n\u001B[37m" + msg + "\u001B[0m"
	} else {
		return "Running \u001B[34m" + cmd + "\u001B[0m to: \u001B[3m" + purpose + "\u001B[0m\n\u001B[37m" + msg + "\u001B[0m"
	}
}

func runYttWithFilesAndStdin(paths []string, stdin io.Reader, logFn func(name string, err error, stderr string, args []string), args ...string) (CmdResult, error) {
	if stdin != nil {
		paths = append(paths, "-")
	}

	cmdArgs := []string{}
	for _, path := range paths {
		cmdArgs = append(cmdArgs, "--file="+path)
	}

	cmdArgs = append(cmdArgs, args...)
	return runCmd("ytt", stdin, cmdArgs, logFn)
}

func filterSlice[T any](slice []T, filterFunc func(v T) bool) []T {
	var result []T
	for _, v := range slice {
		if filterFunc(v) {
			result = append(result, v)
		}
	}
	return result
}

// Concatenates multiple slices of the same type together creating a new underlying array
func concatenate[T any](slices ...[]T) []T {
	result := []T{}
	for _, slice := range slices {
		result = append(result, slice...)
	}
	return result
}

func isExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	log.Error().Err(err).Msg("Unable to stat file")
	return false, err
}

func collectBySubpath(rootDir string, targetDir string, subpath string) []string {
	items := []string{}
	currentPath := rootDir
	levels := []string{""}
	levels = append(levels, strings.Split(targetDir, filepath.FromSlash("/"))...)
	for _, level := range levels {
		currentPath = filepath.Join(currentPath, level)
		item := filepath.Join(currentPath, subpath)
		if _, err := os.Stat(item); err == nil {
			items = append(items, item)
		}
	}
	return items
}

// copyDir copies a directory recursively, overwriting existing files if overwrite is true.
// If overwrite is false, existing files will not be overwritten, an error will be returned instead.
// The destination directory will be created if it does not exist.
func copyDir(src, dst string, overwrite bool) (err error) {
	if err = os.MkdirAll(dst, os.ModePerm); err != nil {
		return
	}

	err = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			if err = os.MkdirAll(dstPath, os.ModePerm); err != nil {
				return err
			}
		} else {
			if !overwrite {
				if _, err = os.Stat(dstPath); err == nil {
					return nil
				}
			}

			if err = copyFile(path, dstPath); err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) (err error) {
	dstFile, err := os.Create(dst)
	if err != nil {
		return
	}
	defer dstFile.Close()

	srcFile, err := os.Open(src)
	if err != nil {
		return
	}
	defer srcFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return
	}

	return nil
}

// mapToSlice converts a map of strings to a slice of strings in the form of key=value
func mapToSlice(env map[string]string) []string {
	var envSlice []string
	for k, v := range env {
		envSlice = append(envSlice, k+"="+v)
	}
	return envSlice
}

func createURLSlug(url string) string {
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	url = strings.ReplaceAll(url, "/", "-")
	return url
}
