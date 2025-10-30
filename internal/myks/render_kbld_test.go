package myks

import (
	"os"
	"path/filepath"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

// TestKbld_generateKbldOverrideConfig tests that generateKbldOverrideConfig
// produces a stable, sorted output regardless of the order of input overrides
func TestKbld_generateKbldOverrideConfig(t *testing.T) {
	// Create a temporary directory for test
	tmpDir := t.TempDir()

	// Create a mock application
	globe := &Globe{
		RootDir: tmpDir,
	}

	env := &Environment{
		ID:  "test-env",
		g:   globe,
		Dir: tmpDir,
	}

	app := &Application{
		Name:      "test-app",
		Prototype: "test-proto",
		e:         env,
	}

	// Create the kbld instance
	kbld := &Kbld{
		ident: "test",
		app:   app,
	}

	tests := []struct {
		name      string
		overrides map[string]string
		wantOrder []string // expected order of images in output
		wantErr   bool
	}{
		{
			name:      "empty overrides",
			overrides: map[string]string{},
			wantOrder: []string{},
			wantErr:   false,
		},
		{
			name: "single override",
			overrides: map[string]string{
				"nginx:latest": "my-registry.local/nginx:latest",
			},
			wantOrder: []string{"nginx:latest"},
			wantErr:   false,
		},
		{
			name: "multiple overrides - sorted alphabetically",
			overrides: map[string]string{
				"redis:7":      "my-registry.local/redis:7",
				"nginx:latest": "my-registry.local/nginx:latest",
				"postgres:15":  "my-registry.local/postgres:15",
				"alpine:3.18":  "my-registry.local/alpine:3.18",
				"ubuntu:22.04": "my-registry.local/ubuntu:22.04",
			},
			wantOrder: []string{
				"alpine:3.18",
				"nginx:latest",
				"postgres:15",
				"redis:7",
				"ubuntu:22.04",
			},
			wantErr: false,
		},
		{
			name: "overrides with special characters",
			overrides: map[string]string{
				"gcr.io/project/image:v1.0.0":          "my-registry.local/project/image:v1.0.0",
				"index.docker.io/library/nginx:latest": "my-registry.local/library/nginx:latest",
				"quay.io/bitnami/redis:7":              "my-registry.local/bitnami/redis:7",
			},
			wantOrder: []string{
				"gcr.io/project/image:v1.0.0",
				"index.docker.io/library/nginx:latest",
				"quay.io/bitnami/redis:7",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate the override config
			filePath, err := kbld.generateKbldOverrideConfig(tt.overrides)

			if (err != nil) != tt.wantErr {
				t.Errorf("generateKbldOverrideConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// If no overrides, file might not be created or be empty
			if len(tt.overrides) == 0 {
				return
			}

			// Read the generated file
			data, err := os.ReadFile(filepath.Clean(filePath))
			if err != nil {
				t.Fatalf("Failed to read generated file: %v", err)
			}

			// Parse the YAML
			type kbldOverride struct {
				Image    string `yaml:"image"`
				NewImage string `yaml:"newImage"`
			}

			type kbldConfig struct {
				APIVersion string         `yaml:"apiVersion"`
				Kind       string         `yaml:"kind"`
				Overrides  []kbldOverride `yaml:"overrides"`
			}

			var config kbldConfig
			if err := yaml.Unmarshal(data, &config); err != nil {
				t.Fatalf("Failed to unmarshal generated config: %v", err)
			}

			// Verify the structure
			if config.APIVersion != "kbld.k14s.io/v1alpha1" {
				t.Errorf("Expected APIVersion 'kbld.k14s.io/v1alpha1', got '%s'", config.APIVersion)
			}

			if config.Kind != "Config" {
				t.Errorf("Expected Kind 'Config', got '%s'", config.Kind)
			}

			// Verify the order
			if len(config.Overrides) != len(tt.wantOrder) {
				t.Errorf("Expected %d overrides, got %d", len(tt.wantOrder), len(config.Overrides))
			}

			for i, override := range config.Overrides {
				if i >= len(tt.wantOrder) {
					break
				}
				if override.Image != tt.wantOrder[i] {
					t.Errorf("Override at index %d: expected image '%s', got '%s'", i, tt.wantOrder[i], override.Image)
				}
				// Verify the mapping is correct
				expectedNewImage, ok := tt.overrides[override.Image]
				if !ok {
					t.Errorf("Override image '%s' not found in original overrides", override.Image)
				}
				if override.NewImage != expectedNewImage {
					t.Errorf("Override at index %d: expected newImage '%s', got '%s'", i, expectedNewImage, override.NewImage)
				}
			}

			// Test idempotency: running the same input multiple times should produce the same output
			filePath2, err := kbld.generateKbldOverrideConfig(tt.overrides)
			if err != nil {
				t.Fatalf("Second generateKbldOverrideConfig() call failed: %v", err)
			}

			data2, err := os.ReadFile(filepath.Clean(filePath2))
			if err != nil {
				t.Fatalf("Failed to read second generated file: %v", err)
			}

			if string(data) != string(data2) {
				t.Errorf("generateKbldOverrideConfig() is not idempotent, produced different output on second call")
			}
		})
	}
}

// TestKbld_generateKbldOverrideConfig_Consistency tests that the same overrides
// produce consistent output regardless of Go's map iteration order
func TestKbld_generateKbldOverrideConfig_Consistency(t *testing.T) {
	// Create a temporary directory for test
	tmpDir := t.TempDir()

	// Create a mock application
	globe := &Globe{
		RootDir: tmpDir,
	}

	env := &Environment{
		ID:  "test-env",
		g:   globe,
		Dir: tmpDir,
	}

	app := &Application{
		Name:      "test-app",
		Prototype: "test-proto",
		e:         env,
	}

	// Create the kbld instance
	kbld := &Kbld{
		ident: "test",
		app:   app,
	}

	// Create a map with multiple overrides
	overrides := map[string]string{
		"redis:7":      "my-registry.local/redis:7",
		"nginx:latest": "my-registry.local/nginx:latest",
		"postgres:15":  "my-registry.local/postgres:15",
		"alpine:3.18":  "my-registry.local/alpine:3.18",
		"ubuntu:22.04": "my-registry.local/ubuntu:22.04",
	}

	// Generate the config multiple times
	var outputs []string
	for i := 0; i < 5; i++ {
		filePath, err := kbld.generateKbldOverrideConfig(overrides)
		if err != nil {
			t.Fatalf("generateKbldOverrideConfig() iteration %d failed: %v", i, err)
		}

		data, err := os.ReadFile(filepath.Clean(filePath))
		if err != nil {
			t.Fatalf("Failed to read generated file iteration %d: %v", i, err)
		}

		outputs = append(outputs, string(data))
	}

	// All outputs should be identical
	firstOutput := outputs[0]
	for i, output := range outputs[1:] {
		if output != firstOutput {
			t.Errorf("Output %d differs from the first output\nFirst:\n%s\n\nOutput %d:\n%s", i+1, firstOutput, i+1, output)
		}
	}
}
