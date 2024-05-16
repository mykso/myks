package myks

import "testing"

func TestCacheNameGen(t *testing.T) {
	helmConfig := map[string]interface{}{
		"helmChart": map[string]interface{}{
			"name":    "test",
			"version": "1.1.0",
		},
	}
	directoryConfig := map[string]interface{}{
		"directory": map[string]interface{}{
			"path": "test",
		},
	}
	gitConfig := map[string]interface{}{
		"git": map[string]interface{}{
			"url": "https://kubernetes.github.io/ingress-nginx",
			"ref": "feature/test",
		},
	}
	unknownConfig := map[string]interface{}{
		"unknown": "test",
	}
	tests := []struct {
		name    string
		config  map[string]interface{}
		want    string
		wantErr bool
	}{
		{"helm", helmConfig, "helm-test-1.1.0-eb485eb68c39202e", false},
		{"directory", directoryConfig, "dir-test-653bffc7e4203260", false},
		{"git", gitConfig, "git-ingress-nginx-feature-test-950426b55fd7cc75", false},
		{"unknown", unknownConfig, "unknown-62c82ff07115bba5", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := genCacheName(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("genCacheName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("genCacheName() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheNamer_Helm(t *testing.T) {
	validVendirConfig := map[string]interface{}{
		"name":    "test",
		"version": "1.1.0",
	}
	vendirConfigWithoutVersion := map[string]interface{}{
		"name": "test",
	}
	vendirConfigWithoutName := map[string]interface{}{
		"version": "1.1.0",
	}

	tests := []struct {
		name         string
		vendirConfig map[string]interface{}
		want         string
		wantErr      bool
	}{
		{"happy path", validVendirConfig, "helm-test-1.1.0-eb485eb68c39202e", false},
		{"missing version", vendirConfigWithoutVersion, "", true},
		{"missing name", vendirConfigWithoutName, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := helmCacheNamer(tt.vendirConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("helmCacheNamer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("helmCacheNamer() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheNamer_Directory(t *testing.T) {
	validVendirConfig := map[string]interface{}{
		"path": "test",
	}
	vendirConfigWithoutPath := map[string]interface{}{
		"name": "test",
	}

	tests := []struct {
		name         string
		vendirConfig map[string]interface{}
		want         string
		wantErr      bool
	}{
		{"happy path", validVendirConfig, "dir-test-653bffc7e4203260", false},
		{"missing path key", vendirConfigWithoutPath, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := directoryCacheNamer(tt.vendirConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("directoryCacheNamer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("directoryCacheNamer() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCacheNamer_Git(t *testing.T) {
	validVendirConfig := map[string]interface{}{
		"url": "https://kubernetes.github.io/ingress-nginx",
		"ref": "feature/test",
	}
	vendirConfigWithoutURL := map[string]interface{}{
		"ref": "main",
	}
	vendirConfigWithoutRef := map[string]interface{}{
		"url": "https://kubernetes.github.io/ingress-nginx",
	}

	tests := []struct {
		name         string
		vendirConfig map[string]interface{}
		want         string
		wantErr      bool
	}{
		{"happy path", validVendirConfig, "git-ingress-nginx-feature-test-950426b55fd7cc75", false},
		{"missing url key", vendirConfigWithoutURL, "", true},
		{"missing ref key", vendirConfigWithoutRef, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gitCacheNamer(tt.vendirConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("gitCacheNamer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("gitCacheNamer() got = %v, want %v", got, tt.want)
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
