package myks

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

func copyFileSystemToPath(source fs.FS, sourcePath, destinationPath string) (err error) {
	if err = os.MkdirAll(destinationPath, 0o750); err != nil { //nolint:gocritic // named return used by closure below
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
			if ferr = os.MkdirAll(destination, 0o750); ferr != nil { //nolint:gocritic // named return used by closure
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

// copyDir copies a directory recursively, overwriting existing files if overwrite is true.
// If overwrite is false, existing files will not be overwritten; existing files are left unchanged
// and no error is returned. The destination directory will be created if it does not exist.
func copyDir(src, dst string, overwrite bool) error {
	if err := os.MkdirAll(dst, 0o750); err != nil {
		return fmt.Errorf("creating destination directory %s: %w", dst, err)
	}

	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("computing relative path: %w", err)
		}

		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			if err = os.MkdirAll(dstPath, 0o750); err != nil {
				return fmt.Errorf("creating directory %s: %w", dstPath, err)
			}
		} else {
			if !overwrite {
				if _, err = os.Stat(dstPath); err == nil {
					return nil
				}
			}

			if err = copyFile(path, dstPath); err != nil {
				return fmt.Errorf("copying file %s: %w", path, err)
			}
		}

		return nil
	})
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating destination file %s: %w", dst, err)
	}
	defer func() {
		if cerr := dstFile.Close(); cerr != nil {
			log.Warn().Err(cerr).Msg("Failed to close destination file")
		}
	}()

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file %s: %w", src, err)
	}
	defer func() {
		if cerr := srcFile.Close(); cerr != nil {
			log.Warn().Err(cerr).Msg("Failed to close source file")
		}
	}()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying %s to %s: %w", src, dst, err)
	}

	return nil
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

func isDir(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.IsDir(), nil
}

func getSubDirs(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	var subDirs []string
	for _, file := range files {
		if file.IsDir() {
			subDirs = append(subDirs, filepath.Join(dir, file.Name()))
		}
	}

	return subDirs, nil
}

// readDirDereferenceLinks reads the directory and dereferences symlinks
func readDirDereferenceLinks(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	var dirs []string
	for _, file := range files {
		fullPath := filepath.Join(dir, file.Name())
		if file.Type()&fs.ModeSymlink != 0 {
			linked, err := os.Readlink(fullPath)
			if err != nil {
				return nil, fmt.Errorf("reading symlink %s: %w", fullPath, err)
			}
			fullPath = filepath.Clean(filepath.Join(dir, linked))
		}
		dirs = append(dirs, fullPath)
	}

	return dirs, nil
}
