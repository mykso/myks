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

// nul is a single NUL byte used as an unambiguous field separator in hashes.
// Writing it between the relative path and file content prevents collisions
// that would otherwise occur when one path is a prefix of another plus content
// (e.g. path="ab" content="cd" vs path="abc" content="d").
var nul = []byte{0}

// hashDirectory computes a deterministic FNV-1a 64-bit hash of an entire
// directory tree. It visits entries in lexicographic order and includes the
// relative path in the hash so that renames are detected. Regular files are
// hashed by content; symlinks are hashed by their link target (not followed,
// to avoid circular-link issues). Other non-regular entries are skipped.
func hashDirectory(dirPath string) (string, error) {
	h := fnv.New64a()
	hasherErr := func(err error) error {
		return fmt.Errorf("writing to hasher: %w", err)
	}

	root, err := os.OpenRoot(dirPath)
	if err != nil {
		return "", fmt.Errorf("opening root directory %s: %w", dirPath, err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("Failed to close root directory")
		}
	}()

	err = filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		path = filepath.Clean(path)

		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return fmt.Errorf("getting relative path for %s: %w", path, err)
		}

		switch {
		case d.IsDir():
			// Skip the root itself; all other directories are hashed by path so
			// that adding/removing empty directories invalidates the hash.
			if relPath == "." {
				return nil
			}
			// Hash: relPath NUL "dir" NUL
			for _, part := range [][]byte{[]byte(relPath), nul, []byte("dir"), nul} {
				if _, err := h.Write(part); err != nil {
					return hasherErr(err)
				}
			}

		case d.Type().IsRegular():
			// Hash: relPath NUL file-content NUL
			if _, err := h.Write([]byte(relPath)); err != nil {
				return hasherErr(err)
			}
			if _, err := h.Write(nul); err != nil {
				return hasherErr(err)
			}
			file, err := root.Open(relPath)
			if err != nil {
				return fmt.Errorf("opening file %s: %w", relPath, err)
			}
			defer func() {
				if closeErr := file.Close(); closeErr != nil {
					log.Error().Err(closeErr).Msg("Failed to close file")
				}
			}()
			if _, err := io.Copy(h, file); err != nil {
				return hasherErr(err)
			}
			if _, err := h.Write(nul); err != nil {
				return hasherErr(err)
			}

		case d.Type()&fs.ModeSymlink != 0:
			// Hash: relPath NUL "symlink:" linkTarget NUL
			// We hash the link target string rather than following it to avoid
			// infinite loops on circular symlinks.
			linkTarget, err := root.Readlink(relPath)
			if err != nil {
				return fmt.Errorf("reading link %s: %w", relPath, err)
			}
			parts := [][]byte{[]byte(relPath), nul, []byte("symlink:" + linkTarget), nul}
			for _, part := range parts {
				if _, err := h.Write(part); err != nil {
					return hasherErr(err)
				}
			}
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to hash directory: %w", err)
	}
	return fmt.Sprintf("%x", h.Sum64()), nil
}
