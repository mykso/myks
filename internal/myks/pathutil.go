package myks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func findSubPath(path, subPath string) (string, bool) {
	index := strings.Index(path, subPath)
	if index == -1 {
		return "", false
	}
	return path[:index+len(subPath)], true
}

func collectBySubpath(rootDir, targetDir, subpath string) []string {
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

func createURLSlug(url string) string {
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "oci://")
	url = strings.ReplaceAll(url, "/", "-")
	return url
}

func ensureValidChartEntry(entryPath string) error {
	if entryPath == "" {
		return fmt.Errorf("empty entry path")
	}

	fileInfo, err := os.Stat(entryPath)
	if err != nil {
		return err
	}
	canonicName := entryPath
	if fileInfo.Mode()&os.ModeSymlink == 1 {
		if name, readErr := os.Readlink(entryPath); readErr != nil {
			return readErr
		} else {
			canonicName = name
		}
	}

	fileInfo, err = os.Stat(canonicName)
	if err != nil {
		return err
	}

	if !fileInfo.IsDir() {
		return fmt.Errorf("non-directory entry")
	}

	if exists, err := isExist(filepath.Join(canonicName, "Chart.yaml")); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("no Chart.yaml found")
	}

	return nil
}
