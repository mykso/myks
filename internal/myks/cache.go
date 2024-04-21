package myks

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

type CacheNamer interface {
	Name(path string, vendirConfig map[string]interface{}) (string, error)
}

func findCacheNamer(config map[string]interface{}) CacheNamer {
	if config["helmChart"] != nil {
		return HelmCacheNamer{}
	}
	if config["directory"] != nil {
		return DirectoryCacheNamer{}
	}
	if config["git"] != nil {
		return GitCacheNamer{}
	}
	return DefaultCacheNamer{}
}

type DefaultCacheNamer struct{}

func (h DefaultCacheNamer) Name(path string, config map[string]interface{}) (string, error) {
	yaml, err := sortYaml(config)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s", filepath.Base(path), hashString(yaml)), nil
}

type HelmCacheNamer struct{}

func (h HelmCacheNamer) Name(_ string, config map[string]interface{}) (string, error) {
	yaml, err := sortYaml(config)
	if err != nil {
		return "", err
	}
	if config["helmChart"] == nil {
		return "", fmt.Errorf("expected vendir config for helm chart, but did not find helmChart yaml key")
	}
	helmChart := config["helmChart"].(map[string]interface{})
	if helmChart["name"] == nil {
		return "", fmt.Errorf("expected name in vendir config for helm chart, but did not find it")
	}
	if helmChart["version"] == nil {
		return "", fmt.Errorf("expected version in vendir config for helm chart, but did not find it")
	}
	chartName := helmChart["name"].(string)
	version := helmChart["version"].(string)
	return fmt.Sprintf("%s-%s-%s-%s", "helm", chartName, version, hashString(yaml)), nil
}

type GitCacheNamer struct{}

func (h GitCacheNamer) Name(_ string, config map[string]interface{}) (string, error) {
	yaml, err := sortYaml(config)
	if err != nil {
		return "", err
	}
	if config["git"] == nil {
		return "", fmt.Errorf("expected vendir config for git, but did not find git yaml key")
	}
	git := config["git"].(map[string]interface{})
	var repoUrl, ref string
	if git["url"] == nil {
		return "", fmt.Errorf("expected url in vendir config for git, but did not find it")
	}
	if git["ref"] == nil {
		return "", fmt.Errorf("expected ref in vendir config for git, but did not find it")
	}
	repoUrl = git["url"].(string)
	ref = git["ref"].(string)
	dir, err := urlSlug(repoUrl)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s-%s-%s", "git", dir, refSlug(ref), hashString(yaml)), nil
}

type DirectoryCacheNamer struct{}

func (h DirectoryCacheNamer) Name(_ string, vendirConfig map[string]interface{}) (string, error) {
	yaml, err := sortYaml(vendirConfig)
	if err != nil {
		return "", err
	}
	if vendirConfig["directory"] == nil {
		return "", fmt.Errorf("expected vendir config for helm chart, but did not find directory yaml key")
	}
	directory := vendirConfig["directory"].(map[string]interface{})
	var path string
	if directory["path"] == nil {
		return "", fmt.Errorf("expected path in vendir config for local directory, but did not find it")
	}
	path = directory["path"].(string)
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

func urlSlug(repoUrl string) (string, error) {
	if len(repoUrl) == 0 {
		return "", nil
	}
	parsedURL, err := url.Parse(repoUrl)
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