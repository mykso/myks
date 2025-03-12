package myks

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

func genCacheName(config map[string]any) (string, error) {
	if val, ok := config["helmChart"]; ok {
		return helmCacheNamer(val.(map[string]any))
	}

	if val, ok := config["directory"]; ok {
		return directoryCacheNamer(val.(map[string]any))
	}

	if val, ok := config["git"]; ok {
		return gitCacheNamer(val.(map[string]any))
	}

	return defaultCacheNamer(config)
}

func defaultCacheNamer(config map[string]any) (string, error) {
	yaml, err := mapToStableString(config)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("unknown-%s", hashString(yaml)), nil
}

func helmCacheNamer(config map[string]any) (string, error) {
	yaml, err := mapToStableString(config)
	if err != nil {
		return "", err
	}
	if config["name"] == nil {
		return "", fmt.Errorf("expected name in vendir config for helm chart, but did not find it")
	}
	if config["version"] == nil {
		return "", fmt.Errorf("expected version in vendir config for helm chart, but did not find it")
	}
	chartName := config["name"].(string)
	version := config["version"].(string)
	return fmt.Sprintf("%s-%s-%s-%s", "helm", chartName, version, hashString(yaml)), nil
}

func gitCacheNamer(config map[string]any) (string, error) {
	yaml, err := mapToStableString(config)
	if err != nil {
		return "", err
	}
	if config["url"] == nil {
		return "", fmt.Errorf("expected url in vendir config for git, but did not find it")
	}
	if config["ref"] == nil {
		return "", fmt.Errorf("expected ref in vendir config for git, but did not find it")
	}
	repoURL := config["url"].(string)
	ref := config["ref"].(string)
	dir, err := urlSlug(repoURL)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%s-%s", "git", dir, refSlug(ref), hashString(yaml)), nil
}

func directoryCacheNamer(config map[string]any) (string, error) {
	yaml, err := mapToStableString(config)
	if err != nil {
		return "", err
	}
	if config["path"] == nil {
		return "", fmt.Errorf("expected path in vendir config for local directory, but did not find it")
	}
	path := config["path"].(string)
	return fmt.Sprintf("%s-%s", directorySlug(path), hashString(yaml)), nil
}

func directorySlug(dirPath string) string {
	if len(dirPath) == 0 {
		return ""
	}
	// we aim for readability in the cache dir here, rather than uniqueness, given that the cache dir name will also
	// include the config digest
	return fmt.Sprintf("%s-%s", "dir", filepath.Base(dirPath))
}

func urlSlug(repoURL string) (string, error) {
	if len(repoURL) == 0 {
		return "", nil
	}
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}
	path := parsedURL.Path
	return filepath.Base(path), nil
}

func refSlug(ref string) string {
	if len(ref) == 0 {
		return ""
	}
	ref = strings.ReplaceAll(ref, "/", "-")
	return filepath.Base(ref)
}
