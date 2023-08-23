package myks

import (
	_ "golang.org/x/exp/slices"
	"strings"
)

// if changes are:
// /path1/path2/path3
// /path1/path3/path3
// then return /path1
func removeSubPaths(paths []string) []string {
	var results []string
	for _, path := range paths {
		if !isSubPath(path, paths) {
			results = append(results, path)
		}
	}
	return results
}

// checks whether path is sub path of any of the paths within paths
func isSubPath(path string, paths []string) bool {
	for _, curPath := range paths {
		if curPath != path {
			if strings.HasPrefix(path, curPath) {
				return true
			}
		}
	}
	return false
}

func removeDuplicates(paths []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range paths {
		if _, exists := seen[item]; !exists {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
