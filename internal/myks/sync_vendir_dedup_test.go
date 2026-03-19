package myks

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mykso/myks/internal/locker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDedupTestApp creates a minimal Application suitable for dedup tests.
func newDedupTestApp(cfg *Config) *Application {
	return &Application{
		Name: "test-app",
		cfg:  cfg,
		e:    &Environment{ID: "test-env"},
	}
}

func TestIsCachePopulated(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &Config{
		ServiceDirName:     ".myks",
		VendirCache:        "vendir-cache",
		VendirLockFileName: "vendir.lock.yaml",
		RootDir:            tmpDir,
	}

	app := newDedupTestApp(cfg)
	v := NewVendirSyncer(locker.NewLocker())

	t.Run("returns false when cache dir does not exist", func(t *testing.T) {
		t.Parallel()
		assert.False(t, v.isCachePopulated(app, "nonexistent-cache"))
	})

	t.Run("returns false when only lock file exists", func(t *testing.T) {
		t.Parallel()
		cacheName := "lock-only-cache"
		cacheDir := app.expandVendirCache(cacheName)
		require.NoError(t, os.MkdirAll(cacheDir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(cacheDir, cfg.VendirLockFileName), []byte("lock"), 0o600))

		assert.False(t, v.isCachePopulated(app, cacheName))
	})

	t.Run("returns false when only data dir exists", func(t *testing.T) {
		t.Parallel()
		cacheName := "data-only-cache"
		cacheDir := app.expandVendirCache(cacheName)
		require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, VendirCacheDataDirName), 0o700))

		assert.False(t, v.isCachePopulated(app, cacheName))
	})

	t.Run("returns true when both lock file and data dir exist", func(t *testing.T) {
		t.Parallel()
		cacheName := "complete-cache"
		cacheDir := app.expandVendirCache(cacheName)
		require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, VendirCacheDataDirName), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(cacheDir, cfg.VendirLockFileName), []byte("lock"), 0o600))

		assert.True(t, v.isCachePopulated(app, cacheName))
	})

	t.Run("returns false when data path is a file not a directory", func(t *testing.T) {
		t.Parallel()
		cacheName := "data-is-file-cache"
		cacheDir := app.expandVendirCache(cacheName)
		require.NoError(t, os.MkdirAll(cacheDir, 0o700))
		// Write a regular file named "data" instead of a directory
		require.NoError(t, os.WriteFile(filepath.Join(cacheDir, VendirCacheDataDirName), []byte("not-a-dir"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(cacheDir, cfg.VendirLockFileName), []byte("lock"), 0o600))

		assert.False(t, v.isCachePopulated(app, cacheName))
	})

	t.Run("returns false when lock path is a directory not a file", func(t *testing.T) {
		t.Parallel()
		cacheName := "lock-is-dir-cache"
		cacheDir := app.expandVendirCache(cacheName)
		require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, VendirCacheDataDirName), 0o700))
		// Create a directory named after the lock file instead of a regular file
		require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, cfg.VendirLockFileName), 0o700))

		assert.False(t, v.isCachePopulated(app, cacheName))
	})
}

func TestSyncCacheEntryWithinRunDedup(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &Config{
		ServiceDirName:       ".myks",
		VendirCache:          "vendir-cache",
		VendirLockFileName:   "vendir.lock.yaml",
		VendirConfigFileName: "vendir.yaml",
		RootDir:              tmpDir,
	}

	app := newDedupTestApp(cfg)
	v := NewVendirSyncer(locker.NewLocker())

	cacheName := "test-cache-dedup"

	// Pre-populate syncedCaches to simulate another goroutine having already synced
	result := &syncResult{done: make(chan struct{})}
	close(result.done) // already completed successfully
	v.syncedCaches.Store(cacheName, result)

	err := v.syncCacheEntry(app, cacheName, "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), v.SyncSkippedInRun.Load())
}

func TestSyncCacheEntryWithinRunDedupPropagatesError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &Config{
		ServiceDirName:       ".myks",
		VendirCache:          "vendir-cache",
		VendirLockFileName:   "vendir.lock.yaml",
		VendirConfigFileName: "vendir.yaml",
		RootDir:              tmpDir,
	}

	app := newDedupTestApp(cfg)
	v := NewVendirSyncer(locker.NewLocker())

	cacheName := "test-cache-error"

	// Pre-populate with a failed result
	result := &syncResult{
		done: make(chan struct{}),
		err:  assert.AnError,
	}
	close(result.done)
	v.syncedCaches.Store(cacheName, result)

	err := v.syncCacheEntry(app, cacheName, "")
	assert.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, int64(1), v.SyncSkippedInRun.Load(), "should count as skipped even when error is propagated")
}

func TestSyncCacheEntryCrossRunDedup(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &Config{
		ServiceDirName:       ".myks",
		VendirCache:          "vendir-cache",
		VendirLockFileName:   "vendir.lock.yaml",
		VendirConfigFileName: "vendir.yaml",
		RootDir:              tmpDir,
	}

	app := newDedupTestApp(cfg)
	v := NewVendirSyncer(locker.NewLocker())

	cacheName := "cross-run-cache"

	// Pre-populate cache directory on disk
	cacheDir := app.expandVendirCache(cacheName)
	require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, VendirCacheDataDirName), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, cfg.VendirLockFileName), []byte("lock"), 0o600))

	// Mark as lazy
	v.lazyCaches.Store(cacheName, true)

	err := v.syncCacheEntry(app, cacheName, "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), v.SyncSkippedCached.Load())
	assert.Equal(t, int64(0), v.SyncExecuted.Load())
}

func TestSyncCacheEntryNoSkipWhenNotLazy(t *testing.T) {
	// Not parallel: uses t.Setenv which requires sequential execution.
	tmpDir := t.TempDir()
	cfg := &Config{
		ServiceDirName:       ".myks",
		VendirCache:          "vendir-cache",
		VendirLockFileName:   "vendir.lock.yaml",
		VendirConfigFileName: "vendir.yaml",
		RootDir:              tmpDir,
	}

	app := newDedupTestApp(cfg)
	v := NewVendirSyncer(locker.NewLocker())

	cacheName := "not-lazy-cache"

	// Pre-populate cache directory on disk
	cacheDir := app.expandVendirCache(cacheName)
	require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, VendirCacheDataDirName), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, cfg.VendirLockFileName), []byte("lock"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, cfg.VendirConfigFileName), []byte(""), 0o600))

	// Mark as NOT lazy (default, but be explicit)
	v.lazyCaches.Store(cacheName, false)

	// Isolate PATH so runVendirSync fails predictably without invoking a real binary.
	t.Setenv("PATH", t.TempDir())

	// This will attempt to run vendir and fail — the important thing is that it does NOT skip
	// due to cache being populated.
	err := v.syncCacheEntry(app, cacheName, "")
	assert.Error(t, err, "expected vendir to fail with no binary on PATH")
	assert.Equal(t, int64(0), v.SyncSkippedCached.Load(), "should not skip when lazy=false")
}

func TestSyncCacheEntryConcurrentDedup(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &Config{
		ServiceDirName:       ".myks",
		VendirCache:          "vendir-cache",
		VendirLockFileName:   "vendir.lock.yaml",
		VendirConfigFileName: "vendir.yaml",
		RootDir:              tmpDir,
	}

	v := NewVendirSyncer(locker.NewLocker())

	cacheName := "concurrent-cache"

	// Pre-populate cache so the "winner" goroutine succeeds via cross-run dedup
	// (avoids needing a real vendir binary)
	app := newDedupTestApp(cfg)
	cacheDir := app.expandVendirCache(cacheName)
	require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, VendirCacheDataDirName), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, cfg.VendirLockFileName), []byte("lock"), 0o600))
	v.lazyCaches.Store(cacheName, true)

	const numGoroutines = 10
	var wg sync.WaitGroup
	var errCount atomic.Int64

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app := newDedupTestApp(cfg)
			if err := v.syncCacheEntry(app, cacheName, ""); err != nil {
				errCount.Add(1)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(0), errCount.Load(), "no errors expected")
	// Exactly one goroutine should hit the cross-run cache skip, the rest hit within-run dedup
	assert.Equal(t, int64(1), v.SyncSkippedCached.Load(), "exactly one cross-run cache skip")
	assert.Equal(t, int64(numGoroutines-1), v.SyncSkippedInRun.Load(), "remaining goroutines should use in-run dedup")
}

func TestVendirDedupStatsBuildSummary(t *testing.T) {
	t.Parallel()

	t.Run("returns empty when no syncs happened", func(t *testing.T) {
		t.Parallel()
		stats := &VendirDedupStats{}
		assert.Empty(t, stats.BuildSummary())
	})

	t.Run("returns summary with all counters", func(t *testing.T) {
		t.Parallel()
		stats := &VendirDedupStats{
			Executed:      3,
			SkippedInRun:  7,
			SkippedCached: 2,
		}

		summary := stats.BuildSummary()
		assert.Contains(t, summary, "Syncs executed:          3")
		assert.Contains(t, summary, "Skipped (in-run dedup):  7")
		assert.Contains(t, summary, "Skipped (cached):        2")
		assert.Contains(t, summary, "Total sync requests:     12")
	})
}
