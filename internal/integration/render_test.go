package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mykso/myks/cmd"
	"github.com/mykso/myks/internal/myks"
)

type testRepo struct {
	name string
	dir  string
}

// findRepos will find all direct subdirectories in provided folder
func findRepos(t *testing.T, basefolder string) []testRepo {
	repos := []testRepo{}
	dir, err := os.Open(basefolder)
	if err != nil {
		t.Errorf("Could not open directory: %s", err)
		return nil
	}

	dirs, err := dir.ReadDir(-1)
	if err != nil {
		t.Errorf("Could not read directories: %s", err)
		return nil
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		repos = append(repos, testRepo{
			name: d.Name(),
			dir:  filepath.Join(basefolder, d.Name()),
		})
	}
	if len(repos) == 0 {
		t.Errorf("Did not find any examples to test")
	}
	return repos
}

func checkCleanGit(t *testing.T) bool {
	t.Helper()
	changes, err := myks.GetChangedFilesGit("")
	if err != nil {
		t.Fatalf("Checking git failed: %s", err)
	}
	if len(changes) > 0 {
		t.Logf("Found changed files: %v", changes)
		t.Errorf("Unexpected changes in git status")
		return false
	}
	return true
}

func chgDir(t *testing.T, base, dir string) {
	err := os.Chdir(filepath.Join(base, dir))
	if err != nil {
		t.Fatalf("Change folder failed: %s", err)
	}
}

func TestRender(t *testing.T) {
	repos := findRepos(t, "../../examples")

	if !checkCleanGit(t) {
		t.Fatal("All changes must be committed before running the integration tests")
	}
	baseFolder, err := os.Getwd()
	if err != nil {
		t.Error(err)
		return
	}
	defer chgDir(t, baseFolder, "")

	for _, repo := range repos {
		t.Run(repo.name, func(t *testing.T) {
			chgDir(t, baseFolder, repo.dir)
			cmd.RunAllCmd()
			if !checkCleanGit(t) {
				t.Log("Commit changes to examples before running this test")
			}
		})
	}
}

func TestInitialRendering(t *testing.T) {
	repos := findRepos(t, "../../examples")

	if !checkCleanGit(t) {
		t.Fatal("All changes must be committed before running the integration tests.")
	}
	baseFolder, err := os.Getwd()
	if err != nil {
		t.Error(err)
		return
	}
	defer chgDir(t, baseFolder, "")

	for _, repo := range repos {
		t.Run(repo.name, func(t *testing.T) {
			chgDir(t, baseFolder, repo.dir)

			err := os.RemoveAll("rendered")
			if err != nil {
				t.Fatalf("Remove rendered directory failed: %s", err)
			}

			cmd.RunAllCmd()
			if !checkCleanGit(t) {
				t.Log("Commit changes to examples before running this test.")
			}
		})
	}
}
