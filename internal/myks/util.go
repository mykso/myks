package myks

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"

	"github.com/rs/zerolog/log"
)

type CmdResult struct {
	Stdout string
	Stderr string
}

func runCmd(name string, stdin io.Reader, args []string) (CmdResult, error) {
	log.Debug().Str("cmd", name).Interface("args", args).Msg("Running command")
	cmd := exec.Command(name, args...)

	if stdin != nil {
		cmd.Stdin = stdin
	}

	var stdoutBs, stderrBs bytes.Buffer
	cmd.Stdout = &stdoutBs
	cmd.Stderr = &stderrBs

	err := cmd.Run()

	return CmdResult{
		Stdout: stdoutBs.String(),
		Stderr: stderrBs.String(),
	}, err
}

func runYttWithFilesAndStdin(paths []string, stdin io.Reader, args ...string) (CmdResult, error) {
	if stdin != nil {
		paths = append(paths, "-")
	}

	cmdArgs := []string{}
	for _, path := range paths {
		cmdArgs = append(cmdArgs, "--file="+path)
	}

	cmdArgs = append(cmdArgs, args...)
	res, err := runCmd("ytt", stdin, cmdArgs)
	if err != nil {
		log.Warn().Str("cmd", "ytt").Interface("args", cmdArgs).Msg("Failed to run command\n" + res.Stderr)
	}

	return res, err
}

func contains(list []string, item string) bool {
	for _, listItem := range list {
		if listItem == item {
			return true
		}
	}
	return false
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
			if err := os.MkdirAll(destination, 0o755); err != nil {
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
