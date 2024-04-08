package myks

import "testing"

func TestHelmCacheNamer_Name(t *testing.T) {
	validVendirConfig := map[string]interface{}{
		"helmChart": map[string]interface{}{
			"name":    "test",
			"version": "1.1.0",
		},
	}
	vendirConfigWithoutVersion := map[string]interface{}{
		"helmChart": map[string]interface{}{
			"name": "test",
		},
	}
	vendirConfigWithoutName := map[string]interface{}{
		"helmChart": map[string]interface{}{
			"version": "1.1.0",
		},
	}
	invalidVendirConfig := map[string]interface{}{
		"nothing": map[string]interface{}{
			"name": "test",
		},
	}

	tests := []struct {
		name         string
		vendirConfig map[string]interface{}
		want         string
		wantErr      bool
	}{
		{"happy path", validVendirConfig, "helm-test-1.1.0-9ca59c856d6df4b6", false},
		{"missing version", vendirConfigWithoutVersion, "", true},
		{"missing name", vendirConfigWithoutName, "", true},
		{"missing helm config", invalidVendirConfig, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &HelmCacheNamer{}
			got, err := h.Name("", tt.vendirConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("Name() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Name() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirectoryCacheNamer_Name(t *testing.T) {
	validVendirConfig := map[string]interface{}{
		"directory": map[string]interface{}{
			"path": "test",
		},
	}
	vendirConfigWithoutPath := map[string]interface{}{
		"directory": map[string]interface{}{
			"name": "test",
		},
	}
	invalidVendirConfig := map[string]interface{}{
		"nothing": map[string]interface{}{
			"name": "test",
		},
	}

	tests := []struct {
		name         string
		vendirConfig map[string]interface{}
		want         string
		wantErr      bool
	}{
		{"happy path", validVendirConfig, "dir-test-5628f4f146ea9abf", false},
		{"missing Directory config", invalidVendirConfig, "", true},
		{"missing path key", vendirConfigWithoutPath, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &DirectoryCacheNamer{}
			got, err := h.Name("", tt.vendirConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("Name() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Name() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitCacheNamer_Name(t *testing.T) {
	validVendirConfig := map[string]interface{}{
		"git": map[string]interface{}{
			"url": "https://kubernetes.github.io/ingress-nginx",
			"ref": "feature/test",
		},
	}
	vendirConfigWithoutUrl := map[string]interface{}{
		"git": map[string]interface{}{
			"ref": "main",
		},
	}
	vendirConfigWithoutRef := map[string]interface{}{
		"git": map[string]interface{}{
			"url": "https://kubernetes.github.io/ingress-nginx",
		},
	}
	invalidVendirConfig := map[string]interface{}{
		"nothing": map[string]interface{}{
			"name": "test",
		},
	}

	tests := []struct {
		name         string
		vendirConfig map[string]interface{}
		want         string
		wantErr      bool
	}{
		{"happy path", validVendirConfig, "git-ingress-nginx-feature-test-9668d10d6d16720b", false},
		{"missing Git config", invalidVendirConfig, "", true},
		{"missing url key", vendirConfigWithoutUrl, "", true},
		{"missing ref key", vendirConfigWithoutRef, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &GitCacheNamer{}
			got, err := h.Name("", tt.vendirConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("Name() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Name() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_directorySlug(t *testing.T) {
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

func Test_urlSlug(t *testing.T) {
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

func Test_refSlug(t *testing.T) {
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
