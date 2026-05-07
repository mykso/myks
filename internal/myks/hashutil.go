package myks

import (
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

func hashString(s string) (string, error) {
	h := fnv.New64a()
	if _, err := h.Write([]byte(s)); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum64()), nil
}

func hashFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}

	defer func() {
		if err := file.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close file")
		}
	}()

	h := fnv.New64a()

	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return fmt.Sprintf("%x", h.Sum64()), nil
}

// hashDirectory computes a deterministic FNV-1a 64-bit hash of an entire
// directory tree. It visits files in lexicographic order and includes the
// relative path of each file in the hash so that renames are detected.
// Symlinks and non-regular files are skipped.
func hashDirectory(dirPath string) (string, error) {
	h := fnv.New64a()
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}
		// Include relative path so that file renames/moves change the hash.
		if _, err := h.Write([]byte(relPath)); err != nil {
			return err
		}
		file, err := os.Open(filepath.Clean(path))
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				log.Error().Err(closeErr).Msg("Failed to close file")
			}
		}()
		if _, err := io.Copy(h, file); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to hash directory: %w", err)
	}
	return fmt.Sprintf("%x", h.Sum64()), nil
}
