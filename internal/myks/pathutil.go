package myks

import (
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
