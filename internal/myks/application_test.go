package myks

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	goyaml "gopkg.in/yaml.v3"
)

func TestApplication_renderDataYaml(t *testing.T) {
	type args struct {
		dataFiles []string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"happy path", args{[]string{"./assets/data-schema.ytt.yaml", "../../testData/ytt/data-file-schema.yaml", "../../testData/ytt/data-file-schema-2.yaml", "../../testData/ytt/data-file-values.yaml"}}, "application:\n  cache:\n    enabled: true\n  name: cert-manager\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := testApp.renderDataYaml(tt.args.dataFiles)
			if (err != nil) != tt.wantErr {
				t.Errorf("renderDataYaml() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !strings.Contains(string(got), tt.want) {
				t.Errorf("renderDataYaml() does not include expected string. got = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestApplication_prototypeDir(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"happy path", "test-app"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := testApp.prototypeDirName()
			if !reflect.DeepEqual(string(got), tt.want) {
				t.Errorf("prototypeDir() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplication_withVendirReadLocks(t *testing.T) {
	tmpDir := t.TempDir()
	g := NewWithDefaults()
	g.RootDir = tmpDir
	cfg := g.Config
	app := &Application{
		Name: "test-app",
		e: &Environment{
			ID:  "env",
			g:   g,
			cfg: &g.Config,
			Dir: "env",
		},
		cfg: &g.Config,
	}

	t.Run("empty map calls fn without locking", func(t *testing.T) {
		// No vendir-links.yaml (or empty) -> getLinksMap returns empty -> fn runs without taking locks
		called := false
		err := app.withVendirReadLocks(func() error {
			called = true
			return nil
		})
		if err != nil {
			t.Fatalf("withVendirReadLocks() err = %v", err)
		}
		if !called {
			t.Error("expected callback to be called when links map is empty")
		}
	})

	t.Run("deduplicates cache entries", func(t *testing.T) {
		serviceDir := filepath.Join(tmpDir, cfg.ServiceDirName, app.e.Dir, cfg.AppsDir, app.Name)
		if err := os.MkdirAll(serviceDir, 0o755); err != nil {
			t.Fatal(err)
		}
		linksMap := map[string]string{
			"path/a": "cache-one",
			"path/b": "cache-one", // same cache -> one lock key
		}
		data, _ := goyaml.Marshal(linksMap)
		if err := os.WriteFile(filepath.Join(serviceDir, cfg.VendirLinksMapFileName), data, 0o644); err != nil {
			t.Fatal(err)
		}
		lockKey := vendirCacheLockKey(app.expandVendirCache("cache-one"), app.cfg.VendirConfigFileName)
		lockHeld := make(chan struct{})
		unlockNow := make(chan struct{})
		readerDone := make(chan struct{})
		go func() {
			mu := getCacheRWMutex(lockKey)
			mu.Lock()
			lockHeld <- struct{}{}
			<-unlockNow
			mu.Unlock()
		}()
		go func() {
			_ = app.withVendirReadLocks(func() error { return nil })
			readerDone <- struct{}{}
		}()
		<-lockHeld
		select {
		case <-readerDone:
			t.Fatal("reader should block while writer holds the same cache mutex")
		case <-time.After(200 * time.Millisecond):
			// Reader is blocking as expected.
		}
		unlockNow <- struct{}{}
		select {
		case <-readerDone:
			// Reader completed after writer released; same mutex was used.
		case <-time.After(time.Second):
			t.Fatal("reader did not complete after writer released lock")
		}
	})

	t.Run("acquires locks in sorted order for multiple keys", func(t *testing.T) {
		env := &Environment{ID: "env", g: g, cfg: &g.Config, Dir: "env"}
		makeApp := func(name string, links map[string]string) *Application {
			a := &Application{Name: name, e: env, cfg: &g.Config}
			serviceDir := filepath.Join(tmpDir, cfg.ServiceDirName, a.e.Dir, cfg.AppsDir, a.Name)
			if err := os.MkdirAll(serviceDir, 0o755); err != nil {
				t.Fatal(err)
			}
			data, _ := goyaml.Marshal(links)
			if err := os.WriteFile(filepath.Join(serviceDir, cfg.VendirLinksMapFileName), data, 0o644); err != nil {
				t.Fatal(err)
			}
			return a
		}
		app1 := makeApp("app1", map[string]string{"path/a": "cache-a", "path/z": "cache-z"})
		app2 := makeApp("app2", map[string]string{"path/z": "cache-z", "path/a": "cache-a"})
		deadline := 2 * time.Second
		var wg sync.WaitGroup
		wg.Add(2)
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		go func() {
			defer wg.Done()
			_ = app1.withVendirReadLocks(func() error { return nil })
		}()
		go func() {
			defer wg.Done()
			_ = app2.withVendirReadLocks(func() error { return nil })
		}()
		select {
		case <-done:
			// Both completed; consistent lock order avoided deadlock.
		case <-time.After(deadline):
			t.Fatal("possible deadlock (lock order regression): both withVendirReadLocks did not complete within " + deadline.String())
		}
	})
}
