package myks

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	vendirconf "carvel.dev/vendir/pkg/vendir/config"
	"github.com/mykso/myks/internal/locker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jn joins path components using the OS path separator (alias for filepath.Join).
func jn(parts ...string) string { return filepath.Join(parts...) }

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

	t.Run("shows config write stats when non-zero", func(t *testing.T) {
		t.Parallel()
		stats := &VendirDedupStats{
			Executed:            1,
			ConfigWriteExecuted: 5,
			ConfigWriteSkipped:  195,
		}

		summary := stats.BuildSummary()
		assert.Contains(t, summary, "Config writes:           5")
		assert.Contains(t, summary, "Config writes skipped:   195")
	})
}

func TestEnsureCacheConfigDedup(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &Config{
		ServiceDirName:       ".myks",
		VendirCache:          "vendir-cache",
		VendirConfigFileName: "vendir.yaml",
		RootDir:              tmpDir,
	}

	v := NewVendirSyncer(locker.NewLocker())

	const cacheName = "test-config-cache"
	config := vendirconf.Config{}

	// Simulate a "previous winner" that already wrote the config
	winnerData, err := config.AsBytes()
	require.NoError(t, err)
	winner := &configResult{done: make(chan struct{}), configData: winnerData}
	close(winner.done)
	v.cacheConfigResults.Store(cacheName, winner)

	app := newDedupTestApp(cfg)
	err = v.ensureCacheConfig(app, cacheName, config)
	require.NoError(t, err)
	assert.Equal(t, int64(1), v.ConfigWriteSkipped.Load(), "should count as skipped when config already written")
	assert.Equal(t, int64(0), v.ConfigWriteExecuted.Load(), "should not count as executed")
}

func TestEnsureCacheConfigConcurrent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &Config{
		ServiceDirName:       ".myks",
		VendirCache:          "vendir-cache",
		VendirConfigFileName: "vendir.yaml",
		RootDir:              tmpDir,
	}

	v := NewVendirSyncer(locker.NewLocker())

	cacheName := "concurrent-config-cache"
	config := vendirconf.Config{
		APIVersion: "vendir.k14s.io/v1alpha1",
		Kind:       "Config",
	}

	// Pre-create the cache directory so ensureCacheConfig can write the vendir config into it.
	cacheDir := (&Application{cfg: cfg, e: &Environment{ID: "test-env"}}).expandVendirCache(cacheName)
	require.NoError(t, os.MkdirAll(cacheDir, 0o700))

	const numGoroutines = 20
	var wg sync.WaitGroup
	var errCount atomic.Int64

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app := newDedupTestApp(cfg)
			if err := v.ensureCacheConfig(app, cacheName, config); err != nil {
				errCount.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(0), errCount.Load(), "no errors expected")
	assert.Equal(t, int64(1), v.ConfigWriteExecuted.Load(), "exactly one config write should execute")
	assert.Equal(t, int64(numGoroutines-1), v.ConfigWriteSkipped.Load(), "remaining goroutines should skip")
}

func TestEnsureCacheConfigMismatchDetected(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &Config{
		ServiceDirName:       ".myks",
		VendirCache:          "vendir-cache",
		VendirConfigFileName: "vendir.yaml",
		RootDir:              tmpDir,
	}

	v := NewVendirSyncer(locker.NewLocker())

	const cacheName = "mismatch-cache"

	// Winner wrote a config without MinimumRequiredVersion
	winnerConfig := vendirconf.Config{
		APIVersion: vendirAPIVersion,
		Kind:       vendirConfigKindConfig,
	}
	winnerData, err := winnerConfig.AsBytes()
	require.NoError(t, err)
	winner := &configResult{done: make(chan struct{}), configData: winnerData}
	close(winner.done)
	v.cacheConfigResults.Store(cacheName, winner)

	// Loser produces a different config (different MinimumRequiredVersion)
	loserConfig := vendirconf.Config{
		APIVersion:             vendirAPIVersion,
		Kind:                   vendirConfigKindConfig,
		MinimumRequiredVersion: "0.29.0",
	}

	app := newDedupTestApp(cfg)
	err = v.ensureCacheConfig(app, cacheName, loserConfig)
	assert.ErrorContains(t, err, "vendir cache config conflict")
	assert.Equal(t, int64(1), v.ConfigWriteSkipped.Load())
}

func TestFindCacheNameForChart(t *testing.T) {
	t.Parallel()

	linksMap := map[string]string{
		jn("charts", "nginx"):      "helm-nginx-1.0.0-abc123",
		jn("charts", "prometheus"): "helm-prometheus-2.0.0-def456",
	}

	t.Run("finds cache name for known chart", func(t *testing.T) {
		t.Parallel()
		result := findCacheNameForChart(linksMap, jn("some", "vendor", "charts", "nginx"), "charts")
		assert.Equal(t, "helm-nginx-1.0.0-abc123", result)
	})

	t.Run("returns empty string for unknown chart", func(t *testing.T) {
		t.Parallel()
		result := findCacheNameForChart(linksMap, jn("some", "vendor", "charts", "unknown"), "charts")
		assert.Empty(t, result)
	})

	t.Run("handles custom helmChartsDirName", func(t *testing.T) {
		t.Parallel()
		customLinksMap := map[string]string{
			jn("helm-charts", "nginx"): "helm-nginx-1.0.0-abc123",
		}
		result := findCacheNameForChart(customLinksMap, jn("vendor", "helm-charts", "nginx"), "helm-charts")
		assert.Equal(t, "helm-nginx-1.0.0-abc123", result)
	})
}

func TestHelmDedupStatsBuildSummary(t *testing.T) {
	t.Parallel()

	t.Run("returns empty when no builds happened", func(t *testing.T) {
		t.Parallel()
		stats := &HelmDedupStats{}
		assert.Empty(t, stats.BuildSummary())
	})

	t.Run("returns summary with executed and skipped", func(t *testing.T) {
		t.Parallel()
		stats := &HelmDedupStats{
			Executed: 3,
			Skipped:  613,
		}
		summary := stats.BuildSummary()
		assert.Contains(t, summary, "Builds executed:         3")
		assert.Contains(t, summary, "Builds skipped:          613")
		assert.Contains(t, summary, "Total build requests:    616")
	})
}
