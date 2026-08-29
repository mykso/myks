package integration_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mykso/myks/cmd"
	"github.com/mykso/myks/internal/myks"
)

// TestMigrateGate validates the `myks migrate` converter with the byte-identical gate
// (config-layer redesign, roadmap step 4): the legacy fixture is copied aside, converted to
// a KCL tree, rendered through the KCL path, and the rendered/ output is compared byte for
// byte against the committed legacy rendering.
func TestMigrateGate(t *testing.T) {
	baseFolder, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer chgDir(t, baseFolder, "")

	legacyRoot := "../../examples/integration-tests"
	schemaPath, err := filepath.Abs(filepath.Join(baseFolder, "..", "..", "kcl", "myks"))
	if err != nil {
		t.Fatal(err)
	}

	// The fixture's vendir configs reference ../_lib, so the copy keeps the examples/ layout.
	scratch := t.TempDir()
	migratedRoot := filepath.Join(scratch, "integration-tests")
	copyFixture(t, filepath.Join(baseFolder, legacyRoot), migratedRoot)
	copyFixture(t, filepath.Join(baseFolder, "..", "..", "examples", "_lib"), filepath.Join(scratch, "_lib"))

	chgDir(t, migratedRoot, "")
	if err := myks.Migrate(myks.New("."), schemaPath, false); err != nil {
		t.Fatalf("Migrate failed: %s", err)
	}
	// Re-running is refused unless --force reads the legacy sources again; the gate below then
	// covers the regenerated tree.
	if err := myks.Migrate(myks.New("."), schemaPath, false); err == nil {
		t.Fatal("Migrate over an already converted repo must fail without force")
	}
	if err := myks.Migrate(myks.New("."), schemaPath, true); err != nil {
		t.Fatalf("Migrate --force failed: %s", err)
	}
	if err := cmd.RenderCmd(myks.New("."), true, true); err != nil {
		t.Fatalf("Render of the migrated repo failed: %s", err)
	}
	chgDir(t, baseFolder, "")

	diffRenderedTrees(t, legacyRoot, migratedRoot, map[string]string{}, normalizeMigratedArgoCDSourcePath)
}

// copyFixture copies the legacy fixture, skipping the service dir and the rendered output
// (both are produced fresh by the migrated repo).
func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".myks" || rel == "rendered" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o750)
		}
		content, err := os.ReadFile(path) // #nosec G304 -- paths come from walking the test fixture
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), content, 0o600)
	})
	if err != nil {
		t.Fatalf("copying fixture: %s", err)
	}
}

// normalizeMigratedArgoCDSourcePath reconciles the only legitimate difference: the migrated
// copy lives outside the git repo, so its ArgoCD Application source paths lack the
// examples/integration-tests/ git path prefix.
func normalizeMigratedArgoCDSourcePath(t *testing.T, relPath string, content []byte) []byte {
	t.Helper()
	if !strings.HasPrefix(relPath, "argocd/") || !strings.HasPrefix(filepath.Base(relPath), "app-") {
		return content
	}
	return bytes.Replace(content,
		[]byte("path: rendered/"),
		[]byte("path: examples/integration-tests/rendered/"), 1)
}
