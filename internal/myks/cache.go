package myks

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	vendirconf "carvel.dev/vendir/pkg/vendir/config"
	yaml "gopkg.in/yaml.v3"
)

func genCacheName(content vendirconf.DirectoryContents) (string, error) { //nolint:gocritic // external type, cannot change
	if content.HelmChart != nil {
		return helmCacheNamer(content)
	}

	if content.Directory != nil {
		return directoryCacheNamer(content)
	}

	if content.Git != nil {
		return gitCacheNamer(content)
	}

	return defaultCacheNamer(content)
}

func contentToStableYAML(content vendirconf.DirectoryContents) (string, error) { //nolint:gocritic // external type
	data, err := yaml.Marshal(content)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func defaultCacheNamer(content vendirconf.DirectoryContents) (string, error) { //nolint:gocritic // external type
	yamlStr, err := contentToStableYAML(content)
	if err != nil {
		return "", err
	}
	hash, err := hashString(yamlStr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("unknown-%s", hash), nil
}

func helmCacheNamer(content vendirconf.DirectoryContents) (string, error) { //nolint:gocritic // external type
	yamlStr, err := contentToStableYAML(content)
	if err != nil {
		return "", err
	}
	chart := content.HelmChart
	if chart.Name == "" {
		return "", fmt.Errorf("expected name in vendir config for helm chart, but did not find it")
	}
	if chart.Version == "" {
		return "", fmt.Errorf("expected version in vendir config for helm chart, but did not find it")
	}
	hash, err := hashString(yamlStr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%s-%s", "helm", chart.Name, chart.Version, hash), nil
}

func gitCacheNamer(content vendirconf.DirectoryContents) (string, error) { //nolint:gocritic // external type
	yamlStr, err := contentToStableYAML(content)
	if err != nil {
		return "", err
	}
	git := content.Git
	if git.URL == "" {
		return "", fmt.Errorf("expected url in vendir config for git, but did not find it")
	}
	if git.Ref == "" {
		return "", fmt.Errorf("expected ref in vendir config for git, but did not find it")
	}
	dir, err := urlSlug(git.URL)
	if err != nil {
		return "", err
	}
	hash, err := hashString(yamlStr)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%s-%s-%s", "git", dir, refSlug(git.Ref), hash), nil
}

func directoryCacheNamer(content vendirconf.DirectoryContents) (string, error) { //nolint:gocritic // external type
	yamlStr, err := contentToStableYAML(content)
	if err != nil {
		return "", err
	}
	dir := content.Directory
	if dir.Path == "" {
		return "", fmt.Errorf("expected path in vendir config for local directory, but did not find it")
	}
	hash, err := hashString(yamlStr)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%s", directorySlug(dir.Path), hash), nil
}

func directorySlug(dirPath string) string {
	if dirPath == "" {
		return ""
	}
	// we aim for readability in the cache dir here, rather than uniqueness, given that the cache dir name will also
	// include the config digest
	return fmt.Sprintf("%s-%s", "dir", filepath.Base(dirPath))
}

func urlSlug(repoURL string) (string, error) {
	if repoURL == "" {
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
	if ref == "" {
		return ""
	}
	ref = strings.ReplaceAll(ref, "/", "-")
	return filepath.Base(ref)
}
