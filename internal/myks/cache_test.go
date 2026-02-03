package myks

import (
	"testing"

	vendirconf "carvel.dev/vendir/pkg/vendir/config"
)

func TestCacheNameGen(t *testing.T) {
	helmConfig := vendirconf.DirectoryContents{
		HelmChart: &vendirconf.DirectoryContentsHelmChart{
			Name:    "test",
			Version: "1.1.0",
		},
	}
	directoryConfig := vendirconf.DirectoryContents{
		Directory: &vendirconf.DirectoryContentsDirectory{
			Path: "test",
		},
	}
	gitConfig := vendirconf.DirectoryContents{
		Git: &vendirconf.DirectoryContentsGit{
			URL: "https://kubernetes.github.io/ingress-nginx",
			Ref: "feature/test",
		},
	}
	unknownConfig := vendirconf.DirectoryContents{
		Path: "some-path",
	}
	tests := []struct {
		name    string
		config  vendirconf.DirectoryContents
		wantErr bool
	}{
		{"helm", helmConfig, false},
		{"directory", directoryConfig, false},
		{"git", gitConfig, false},
		{"unknown", unknownConfig, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := genCacheName(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("genCacheName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got == "" {
				t.Errorf("genCacheName() returned empty string")
			}
		})
	}
}

func TestCacheNamer_Helm(t *testing.T) {
	validContent := vendirconf.DirectoryContents{
		HelmChart: &vendirconf.DirectoryContentsHelmChart{
			Name:    "test",
			Version: "1.1.0",
		},
	}
	contentWithoutVersion := vendirconf.DirectoryContents{
		HelmChart: &vendirconf.DirectoryContentsHelmChart{
			Name: "test",
		},
	}
	contentWithoutName := vendirconf.DirectoryContents{
		HelmChart: &vendirconf.DirectoryContentsHelmChart{
			Version: "1.1.0",
		},
	}

	tests := []struct {
		name    string
		content vendirconf.DirectoryContents
		wantErr bool
	}{
		{"happy path", validContent, false},
		{"missing version", contentWithoutVersion, true},
		{"missing name", contentWithoutName, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := helmCacheNamer(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("helmCacheNamer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == "" {
				t.Errorf("helmCacheNamer() returned empty string")
			}
		})
	}
}

func TestCacheNamer_Directory(t *testing.T) {
	validContent := vendirconf.DirectoryContents{
		Directory: &vendirconf.DirectoryContentsDirectory{
			Path: "test",
		},
	}
	contentWithoutPath := vendirconf.DirectoryContents{
		Directory: &vendirconf.DirectoryContentsDirectory{},
	}

	tests := []struct {
		name    string
		content vendirconf.DirectoryContents
		wantErr bool
	}{
		{"happy path", validContent, false},
		{"missing path key", contentWithoutPath, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := directoryCacheNamer(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("directoryCacheNamer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == "" {
				t.Errorf("directoryCacheNamer() returned empty string")
			}
		})
	}
}

func TestCacheNamer_Git(t *testing.T) {
	validContent := vendirconf.DirectoryContents{
		Git: &vendirconf.DirectoryContentsGit{
			URL: "https://kubernetes.github.io/ingress-nginx",
			Ref: "feature/test",
		},
	}
	contentWithoutURL := vendirconf.DirectoryContents{
		Git: &vendirconf.DirectoryContentsGit{
			Ref: "main",
		},
	}
	contentWithoutRef := vendirconf.DirectoryContents{
		Git: &vendirconf.DirectoryContentsGit{
			URL: "https://kubernetes.github.io/ingress-nginx",
		},
	}

	tests := []struct {
		name    string
		content vendirconf.DirectoryContents
		wantErr bool
	}{
		{"happy path", validContent, false},
		{"missing url key", contentWithoutURL, true},
		{"missing ref key", contentWithoutRef, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gitCacheNamer(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("gitCacheNamer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == "" {
				t.Errorf("gitCacheNamer() returned empty string")
			}
		})
	}
}

func TestCacheName_directorySlug(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"happy path", "/test/me/", "dir-me"},
		{"no slashes", "test/me", "dir-me"},
		{"local reference", "../test/me", "dir-me"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := directorySlug(tt.path); got != tt.want {
				t.Errorf("directorySlug() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheName_urlSlug(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"happy path", "https://github.com/kubernetes/ingress-nginx", "ingress-nginx"},
		{"http", "http://github.com/kubernetes/ingress-nginx/", "ingress-nginx"},
		{"long url", "https://github.com/kubernetes/some-folder/ingress-nginx/", "ingress-nginx"},
		{"trailing slash", "http://github.com/kubernetes/ingress-nginx", "ingress-nginx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := urlSlug(tt.url); got != tt.want {
				t.Errorf("urlSlug() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheName_refSlug(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{"happy path", "1.1.0", "1.1.0"},
		{"slashes", "feature/test", "feature-test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := refSlug(tt.ref); got != tt.want {
				t.Errorf("urlSlug() = %v, want %v", got, tt.want)
			}
		})
	}
}
