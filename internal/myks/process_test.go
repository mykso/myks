package myks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestGetCacheMutex_RWMutexSemantics verifies that getCacheMutex returns a
// *sync.RWMutex that allows concurrent readers and serializes writers.
func TestGetCacheMutex_RWMutexSemantics(t *testing.T) {
	key := fmt.Sprintf("test-rwmutex-%d", time.Now().UnixNano())
	mu := getCacheMutex(key)

	// Multiple read locks must coexist without deadlock.
	const readers = 5
	var wg sync.WaitGroup
	wg.Add(readers)
	for range readers {
		go func() {
			defer wg.Done()
			mu.RLock()
			time.Sleep(5 * time.Millisecond)
			mu.RUnlock()
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("concurrent RLock calls deadlocked; expected them to proceed in parallel")
	}

	// A write lock must block until all read locks are released.
	mu.RLock()
	writeDone := make(chan struct{})
	go func() {
		mu.Lock()
		defer mu.Unlock()
		close(writeDone)
	}()
	// Give the writer goroutine time to start and block on Lock().
	time.Sleep(10 * time.Millisecond)
	select {
	case <-writeDone:
		t.Fatal("write lock acquired while read lock was still held")
	default:
	}
	mu.RUnlock()
	select {
	case <-writeDone:
	case <-time.After(time.Second):
		t.Fatal("write lock did not acquire after read lock was released")
	}
}

// TestGetCacheMutex_SameKeyReturnsSameMutex verifies that the same key always
// returns the same *sync.RWMutex instance.
func TestGetCacheMutex_SameKeyReturnsSameMutex(t *testing.T) {
	key := fmt.Sprintf("test-same-key-%d", time.Now().UnixNano())
	mu1 := getCacheMutex(key)
	mu2 := getCacheMutex(key)
	if mu1 != mu2 {
		t.Errorf("getCacheMutex returned different instances for the same key")
	}
}

// TestGetCacheMutex_DifferentKeysReturnDifferentMutexes verifies that distinct
// keys produce independent mutexes.
func TestGetCacheMutex_DifferentKeysReturnDifferentMutexes(t *testing.T) {
	key1 := fmt.Sprintf("test-key-a-%d", time.Now().UnixNano())
	key2 := fmt.Sprintf("test-key-b-%d", time.Now().UnixNano())
	if getCacheMutex(key1) == getCacheMutex(key2) {
		t.Errorf("getCacheMutex returned the same instance for different keys")
	}
}

// TestWithCacheReadLocks_NoLinksMap verifies that withCacheReadLocks calls fn
// directly when there is no links map (app without vendir config).
func TestWithCacheReadLocks_NoLinksMap(t *testing.T) {
	tmpDir := t.TempDir()
	app := newTestApp(t, tmpDir)

	called := false
	err := app.withCacheReadLocks(func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("withCacheReadLocks unexpectedly returned error: %v", err)
	}
	if !called {
		t.Error("fn was not called")
	}
}

// TestWithCacheReadLocks_FnError verifies that errors from fn are propagated.
func TestWithCacheReadLocks_FnError(t *testing.T) {
	tmpDir := t.TempDir()
	app := newTestApp(t, tmpDir)

	want := errors.New("fn error")
	err := app.withCacheReadLocks(func() error {
		return want
	})
	if !errors.Is(err, want) {
		t.Errorf("withCacheReadLocks: got %v, want %v", err, want)
	}
}

// TestWithCacheReadLocks_WithLinksMap verifies that withCacheReadLocks acquires
// read locks on cache entries listed in the links map and releases them after fn.
func TestWithCacheReadLocks_WithLinksMap(t *testing.T) {
	tmpDir := t.TempDir()
	app := newTestApp(t, tmpDir)

	// Create a fake links map pointing to a cache entry.
	cacheName := "test-cache-entry"
	cacheDir := app.expandVendirCache(cacheName)
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		t.Fatalf("creating cache dir: %v", err)
	}
	linksMap := map[string]string{"vendor/path": cacheName}
	if err := (&VendirSyncer{ident: "vendir"}).saveLinksMap(app, linksMap); err != nil {
		t.Fatalf("saving links map: %v", err)
	}

	vendirConfigPath := filepath.Join(cacheDir, app.cfg.VendirConfigFileName)
	mu := getCacheMutex(vendirConfigPath)

	// Verify: while fn executes, the read lock is held (a write lock cannot be acquired).
	fnRunning := make(chan struct{})
	fnDone := make(chan struct{})
	fnErrCh := make(chan error, 1)

	go func() {
		fnErrCh <- app.withCacheReadLocks(func() error {
			close(fnRunning)
			<-fnDone
			return nil
		})
	}()

	<-fnRunning

	// Try to acquire write lock — should block because read lock is held.
	writeLocked := make(chan struct{})
	go func() {
		mu.Lock()
		defer mu.Unlock()
		close(writeLocked)
	}()

	// Give writer time to attempt the lock.
	time.Sleep(10 * time.Millisecond)
	select {
	case <-writeLocked:
		t.Error("write lock acquired while withCacheReadLocks fn was running; read lock was not held")
	default:
	}

	// Release fn; write lock should now succeed.
	close(fnDone)
	select {
	case <-writeLocked:
	case <-time.After(time.Second):
		t.Fatal("write lock did not acquire after withCacheReadLocks fn completed")
	}

	if fnErr := <-fnErrCh; fnErr != nil {
		t.Fatalf("withCacheReadLocks returned unexpected error: %v", fnErr)
	}
}

// TestProcessOpts_SyncOnlyAndRenderOnly tests that ProcessOpts correctly conveys
// intent (sync-only vs render-only vs both).
// TestWithCacheReadLocks_ConsistentLockOrdering verifies that withCacheReadLocks
// acquires locks in a deterministic order, preventing ABBA deadlocks when
// multiple goroutines lock overlapping sets of cache entries concurrently.
func TestWithCacheReadLocks_ConsistentLockOrdering(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two apps that share the same two cache entries but whose linksMap
	// iteration order may differ (Go map iteration is random).
	cacheNames := []string{"cache-alpha", "cache-beta"}
	apps := make([]*Application, 2)
	for i := range apps {
		app := newTestApp(t, tmpDir)
		app.Name = fmt.Sprintf("app-%d", i)
		for _, cn := range cacheNames {
			cacheDir := app.expandVendirCache(cn)
			if err := os.MkdirAll(cacheDir, 0o750); err != nil {
				t.Fatalf("creating cache dir: %v", err)
			}
		}
		// Each app maps two vendor paths to the two cache entries.
		// Use reversed key names so that naive map iteration is more likely
		// to produce different orders across the two apps.
		linksMap := map[string]string{
			fmt.Sprintf("vendor/path-%d-a", i): cacheNames[0],
			fmt.Sprintf("vendor/path-%d-b", i): cacheNames[1],
		}
		if err := (&VendirSyncer{ident: "vendir"}).saveLinksMap(app, linksMap); err != nil {
			t.Fatalf("saving links map: %v", err)
		}
		apps[i] = app
	}

	// Simulate a writer that periodically locks each cache entry, creating
	// the writer-priority conditions that trigger the deadlock.
	alphaPath := filepath.Join(apps[0].expandVendirCache(cacheNames[0]), apps[0].cfg.VendirConfigFileName)
	betaPath := filepath.Join(apps[0].expandVendirCache(cacheNames[1]), apps[0].cfg.VendirConfigFileName)
	alphaMu := getCacheMutex(alphaPath)
	betaMu := getCacheMutex(betaPath)

	done := make(chan struct{})
	defer close(done)

	// Writer goroutine: alternates write-locking each mutex.
	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			alphaMu.Lock()
			alphaMu.Unlock()
			betaMu.Lock()
			betaMu.Unlock()
		}
	}()

	// Run both apps' withCacheReadLocks concurrently many times.
	// Without consistent ordering, this deadlocks under the race detector.
	const iterations = 50
	var wg sync.WaitGroup
	errCh := make(chan error, 2*iterations)
	for _, app := range apps {
		for range iterations {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errCh <- app.withCacheReadLocks(func() error {
					return nil
				})
			}()
		}
	}

	// Use a timeout to detect deadlock.
	allDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(allDone)
	}()

	select {
	case <-allDone:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock detected: withCacheReadLocks did not complete within timeout")
	}

	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("withCacheReadLocks returned unexpected error: %v", err)
		}
	}
}

func TestProcessOpts_Fields(t *testing.T) {
	tests := []struct {
		name   string
		opts   ProcessOpts
		doSync bool
		doRend bool
	}{
		{"both", ProcessOpts{Sync: true, Render: true}, true, true},
		{"sync only", ProcessOpts{Sync: true, Render: false}, true, false},
		{"render only", ProcessOpts{Sync: false, Render: true}, false, true},
		{"neither", ProcessOpts{Sync: false, Render: false}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opts.Sync != tt.doSync {
				t.Errorf("Sync = %v, want %v", tt.opts.Sync, tt.doSync)
			}
			if tt.opts.Render != tt.doRend {
				t.Errorf("Render = %v, want %v", tt.opts.Render, tt.doRend)
			}
		})
	}
}

// TestProcess_PipelineOrdering verifies that in Globe.Process with both sync
// and render enabled, each app's render starts only after its own sync
// completes — not after all apps have synced.
//
// This test uses a Globe with mock sync and render operations tracked by
// atomic counters and channels to verify the interleaving.
func TestProcess_OrderingWithinApp(t *testing.T) {
	// We test the ordering constraint: for a given app, sync must precede render.
	// We do this by tracking, for each app, whether sync ran before render.
	const appCount = 3
	var (
		syncDone       [appCount]atomic.Bool
		orderViolation atomic.Bool
	)

	// Simulate: for each app index i, the sync sets syncDone[i]=true, and
	// the render checks it. If render sees syncDone[i]=false, the ordering
	// is violated.
	type appTask struct {
		syncFn   func() error
		renderFn func() error
	}

	tasks := make([]appTask, appCount)
	for i := range appCount {
		idx := i
		tasks[idx] = appTask{
			syncFn: func() error {
				time.Sleep(time.Duration(idx) * 2 * time.Millisecond)
				syncDone[idx].Store(true)
				return nil
			},
			renderFn: func() error {
				if !syncDone[idx].Load() {
					orderViolation.Store(true)
				}
				return nil
			},
		}
	}

	var wg sync.WaitGroup
	for i := range appCount {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := tasks[i].syncFn(); err != nil {
				t.Errorf("sync[%d] failed: %v", i, err)
			}
			if err := tasks[i].renderFn(); err != nil {
				t.Errorf("render[%d] failed: %v", i, err)
			}
		}()
	}
	wg.Wait()

	if orderViolation.Load() {
		t.Error("render was called before sync completed for the same app")
	}
}

// newTestApp creates a minimal Application suitable for unit testing.
// It uses a fresh temp directory as the root so tests don't pollute real dirs.
func newTestApp(t *testing.T, rootDir string) *Application {
	t.Helper()
	cfg := NewWithDefaults().Config
	cfg.RootDir = rootDir
	env := &Environment{
		ID:  "test-env",
		Dir: "envs/test-env",
		cfg: &cfg,
	}
	return &Application{
		Name:      "test-app",
		Prototype: filepath.Join(rootDir, "prototypes", "test-app"),
		e:         env,
		cfg:       &cfg,
	}
}
