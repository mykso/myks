package myks

import (
	"testing"
)

func TestKbldConfig_applyOverrides(t *testing.T) {
	tests := []struct {
		name     string
		config   KbldConfig
		imageRef string
		want     string
		wantErr  bool
	}{
		{
			name: "no overrides",
			config: KbldConfig{
				Overrides: []ImageRefOverride{},
			},
			imageRef: "nginx:latest",
			want:     "",
			wantErr:  false,
		},
		{
			name: "change registry for Docker Hub image",
			config: KbldConfig{
				Overrides: []ImageRefOverride{
					{
						Match: ImageRefPattern{
							Registry: "index\\.docker\\.io",
						},
						Replace: ImageRefPattern{
							Registry: "my-registry.local",
						},
					},
				},
			},
			imageRef: "nginx:latest",
			want:     "my-registry.local/library/nginx:latest",
			wantErr:  false,
		},
		{
			name: "change registry and repository for Bitnami image",
			config: KbldConfig{
				Overrides: []ImageRefOverride{
					{
						Match: ImageRefPattern{
							Repository: "bitnami/(.+)",
						},
						Replace: ImageRefPattern{
							Registry:   "my-registry.local",
							Repository: "bitnamilegacy/$1",
						},
					},
				},
			},
			imageRef: "bitnami/nginx:1.25.0",
			want:     "my-registry.local/bitnamilegacy/nginx:1.25.0",
			wantErr:  false,
		},
		{
			name: "change tag pattern",
			config: KbldConfig{
				Overrides: []ImageRefOverride{
					{
						Match: ImageRefPattern{
							Tag: "latest",
						},
						Replace: ImageRefPattern{
							Tag: "stable",
						},
					},
				},
			},
			imageRef: "nginx:latest",
			want:     "index.docker.io/library/nginx:stable",
			wantErr:  false,
		},
		{
			name: "implicit latest tag",
			config: KbldConfig{
				Overrides: []ImageRefOverride{
					{
						Match: ImageRefPattern{
							Tag: "latest",
						},
						Replace: ImageRefPattern{
							Tag: "stable",
						},
					},
				},
			},
			imageRef: "nginx",
			want:     "index.docker.io/library/nginx:stable",
			wantErr:  false,
		},
		{
			name: "multiple overrides - first match wins",
			config: KbldConfig{
				Overrides: []ImageRefOverride{
					{
						Match: ImageRefPattern{
							Repository: "library/nginx",
						},
						Replace: ImageRefPattern{
							Registry: "first-registry.local",
						},
					},
					{
						Match: ImageRefPattern{
							Repository: "library/nginx",
						},
						Replace: ImageRefPattern{
							Registry: "second-registry.local",
						},
					},
				},
			},
			imageRef: "nginx:latest",
			want:     "first-registry.local/library/nginx:latest",
			wantErr:  false,
		},
		{
			name: "no match - return original",
			config: KbldConfig{
				Overrides: []ImageRefOverride{
					{
						Match: ImageRefPattern{
							Repository: "redis",
						},
						Replace: ImageRefPattern{
							Registry: "my-registry.local",
						},
					},
				},
			},
			imageRef: "nginx:latest",
			want:     "",
			wantErr:  false,
		},
		{
			name: "explicit docker.io registry",
			config: KbldConfig{
				Overrides: []ImageRefOverride{
					{
						Match: ImageRefPattern{
							Registry: "index\\.docker\\.io",
						},
						Replace: ImageRefPattern{
							Registry: "my-registry.local",
						},
					},
				},
			},
			imageRef: "docker.io/library/nginx:latest",
			want:     "my-registry.local/library/nginx:latest",
			wantErr:  false,
		},
		{
			name: "image with digest",
			config: KbldConfig{
				Overrides: []ImageRefOverride{
					{
						Match: ImageRefPattern{
							Registry: "index\\.docker\\.io",
						},
						Replace: ImageRefPattern{
							Registry: "my-registry.local",
						},
					},
				},
			},
			imageRef: "nginx@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			want:     "my-registry.local/library/nginx@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			wantErr:  false,
		},
		{
			name: "private registry image",
			config: KbldConfig{
				Overrides: []ImageRefOverride{
					{
						Match: ImageRefPattern{
							Registry: "gcr\\.io",
						},
						Replace: ImageRefPattern{
							Registry: "my-registry.local",
						},
					},
				},
			},
			imageRef: "gcr.io/myproject/myimage:v1.0.0",
			want:     "my-registry.local/myproject/myimage:v1.0.0",
			wantErr:  false,
		},
		{
			name: "capture group in repository",
			config: KbldConfig{
				Overrides: []ImageRefOverride{
					{
						Match: ImageRefPattern{
							Registry:   "index\\.docker\\.io",
							Repository: "(.+)/(.+)",
						},
						Replace: ImageRefPattern{
							Registry:   "my-registry.local",
							Repository: "mirror/$1/$2",
						},
					},
				},
			},
			imageRef: "myorg/myapp:latest",
			want:     "my-registry.local/mirror/myorg/myapp:latest",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.initOverrides(); err != nil {
				t.Fatalf("initOverrides() error = %v", err)
			}
			got, err := tt.config.applyOverrides(tt.imageRef)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyOverrides() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("applyOverrides() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageRefOverride_apply(t *testing.T) {
	tests := []struct {
		name           string
		override       ImageRefOverride
		registry       string
		repository     string
		tag            string
		wantRegistry   string
		wantRepository string
		wantTag        string
	}{
		{
			name: "replace all fields",
			override: ImageRefOverride{
				Match: ImageRefPattern{
					Registry:   "index\\.docker\\.io",
					Repository: "library/nginx",
					Tag:        "latest",
				},
				Replace: ImageRefPattern{
					Registry:   "my-registry.local",
					Repository: "mirror/nginx",
					Tag:        "stable",
				},
			},
			registry:       "index.docker.io",
			repository:     "library/nginx",
			tag:            "latest",
			wantRegistry:   "my-registry.local",
			wantRepository: "mirror/nginx",
			wantTag:        "stable",
		},
		{
			name: "replace only registry",
			override: ImageRefOverride{
				Replace: ImageRefPattern{
					Registry: "my-registry.local",
				},
			},
			registry:       "index.docker.io",
			repository:     "library/nginx",
			tag:            "latest",
			wantRegistry:   "my-registry.local",
			wantRepository: "library/nginx",
			wantTag:        "latest",
		},
		{
			name: "capture group substitution",
			override: ImageRefOverride{
				Match: ImageRefPattern{
					Repository: "bitnami/(.+)",
				},
				Replace: ImageRefPattern{
					Repository: "mirror/$1",
				},
			},
			registry:       "index.docker.io",
			repository:     "bitnami/nginx",
			tag:            "latest",
			wantRegistry:   "index.docker.io",
			wantRepository: "mirror/nginx",
			wantTag:        "latest",
		},
		{
			name: "no replacement specified",
			override: ImageRefOverride{
				Match: ImageRefPattern{
					Registry: "index\\.docker\\.io",
				},
				Replace: ImageRefPattern{},
			},
			registry:       "index.docker.io",
			repository:     "library/nginx",
			tag:            "latest",
			wantRegistry:   "index.docker.io",
			wantRepository: "library/nginx",
			wantTag:        "latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.override.init(); err != nil {
				t.Fatalf("init override error = %v", err)
			}

			gotRegistry, gotRepository, gotTag := tt.override.apply(tt.registry, tt.repository, tt.tag)
			if gotRegistry != tt.wantRegistry {
				t.Errorf("apply() registry = %v, want %v", gotRegistry, tt.wantRegistry)
			}
			if gotRepository != tt.wantRepository {
				t.Errorf("apply() repository = %v, want %v", gotRepository, tt.wantRepository)
			}
			if gotTag != tt.wantTag {
				t.Errorf("apply() tag = %v, want %v", gotTag, tt.wantTag)
			}
		})
	}
}
