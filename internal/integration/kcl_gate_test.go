package integration_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mykso/myks/cmd"
	"github.com/mykso/myks/internal/myks"
	yaml "gopkg.in/yaml.v3"
)

// TestKclGate is the byte-identical migration gate (config-layer redesign, roadmap step 3):
// the same logical repo rendered in legacy mode and in KCL mode must produce identical
// rendered/ trees. It doubles as the test suite of the future `myks migrate` converter.
func TestKclGate(t *testing.T) {
	baseFolder, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer chgDir(t, baseFolder, "")

	legacyRoot := "../../examples/integration-tests"
	kclRoot := "../../examples/kcl-integration-tests"

	renderRepo(t, baseFolder, legacyRoot)
	renderRepo(t, baseFolder, kclRoot)

	diffRenderedTrees(t, legacyRoot, kclRoot,
		// Per-app waivers: app name -> reason. Waived apps are expected to differ because a
		// KCL derivation intentionally changes values; their outputs are excluded from the diff.
		map[string]string{},
		// The repos live in different directories, so ArgoCD Application source paths legitimately differ.
		normalizeArgoCDSourcePath,
	)
}

func renderRepo(t *testing.T, baseFolder, root string) {
	t.Helper()
	chgDir(t, baseFolder, root)
	if err := cmd.RenderCmd(myks.New("."), true, true); err != nil {
		t.Fatalf("Render of %s failed: %s", root, err)
	}
	chgDir(t, baseFolder, "")
}

// diffRenderedTrees compares the rendered/ trees of two repo roots byte for byte.
// Waived applications are excluded from the comparison; normalize (optional) is applied to
// files of the second tree before comparing.
func diffRenderedTrees(t *testing.T, wantRoot, gotRoot string, waivedApps map[string]string, normalize func(*testing.T, string, []byte) []byte) {
	t.Helper()

	for app, reason := range waivedApps {
		t.Logf("Waived app %q: %s", app, reason)
	}

	wantFiles := collectRenderedFiles(t, wantRoot, waivedApps)
	gotFiles := collectRenderedFiles(t, gotRoot, waivedApps)

	for path, want := range wantFiles {
		got, ok := gotFiles[path]
		if !ok {
			t.Errorf("missing rendered file: %s", path)
			continue
		}
		if normalize != nil {
			got = normalize(t, path, got)
		}
		if !bytes.Equal(want, got) {
			t.Errorf("rendered file differs: %s\n--- %s\n%s\n--- %s\n%s", path, wantRoot, want, gotRoot, got)
		}
	}
	for path := range gotFiles {
		if _, ok := wantFiles[path]; !ok {
			t.Errorf("unexpected rendered file: %s", path)
		}
	}
}

// normalizeArgoCDSourcePath reconciles the fixture-root difference only in an ArgoCD
// Application's spec.source.path. Every other rendered value remains byte-compared.
func normalizeArgoCDSourcePath(t *testing.T, relPath string, content []byte) []byte {
	t.Helper()
	if !strings.HasPrefix(relPath, "argocd/") || !strings.HasPrefix(filepath.Base(relPath), "app-") {
		return content
	}

	var application struct {
		Kind string `yaml:"kind"`
		Spec struct {
			Source struct {
				Path string `yaml:"path"`
			} `yaml:"source"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(content, &application); err != nil {
		t.Fatalf("parsing ArgoCD Application %s: %s", relPath, err)
	}
	if application.Kind != "Application" || !strings.HasPrefix(application.Spec.Source.Path, "examples/kcl-integration-tests/") {
		return content
	}

	got := []byte("path: " + application.Spec.Source.Path)
	if bytes.Count(content, got) != 1 {
		t.Fatalf("finding unique spec.source.path in %s", relPath)
	}
	want := []byte("path: " + strings.Replace(application.Spec.Source.Path, "examples/kcl-integration-tests/", "examples/integration-tests/", 1))
	return bytes.Replace(content, got, want, 1)
}

func TestNormalizeArgoCDSourcePath(t *testing.T) {
	t.Parallel()
	content := []byte(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    repository: examples/kcl-integration-tests/
spec:
  source:
    path: examples/kcl-integration-tests/rendered/envs/dev/app
`)

	got := normalizeArgoCDSourcePath(t, "argocd/dev/app-app.yaml", content)
	want := []byte(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    repository: examples/kcl-integration-tests/
spec:
  source:
    path: examples/integration-tests/rendered/envs/dev/app
`)
	if !bytes.Equal(got, want) {
		t.Errorf("normalized content differs:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// collectRenderedFiles maps rendered/-relative file paths to their content, skipping waived apps.
// Waived paths are rendered/envs/<env>/<app>/** and rendered/argocd/<env>/app-<app>.yaml.
func collectRenderedFiles(t *testing.T, root string, waivedApps map[string]string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	renderedDir := filepath.Join(root, "rendered")
	err := filepath.WalkDir(renderedDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(renderedDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if isWaivedPath(rel, waivedApps) {
			return nil
		}
		content, err := os.ReadFile(path) // #nosec G304 -- paths come from walking the test fixture
		if err != nil {
			return err
		}
		files[rel] = content
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %s", renderedDir, err)
	}
	return files
}

func isWaivedPath(relPath string, waivedApps map[string]string) bool {
	parts := strings.Split(relPath, "/")
	if len(parts) < 3 {
		return false
	}
	for app := range waivedApps {
		switch parts[0] {
		case "envs":
			if parts[2] == app {
				return true
			}
		case "argocd":
			if parts[2] == "app-"+app+".yaml" {
				return true
			}
		}
	}
	return false
}
