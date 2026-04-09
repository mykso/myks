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

func TestBuildChartOnceWithinRunDedup(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &Config{
		ServiceDirName: ".myks",
		VendirCache:    "vendir-cache",
		RootDir:        tmpDir,
	}

	app := newDedupTestApp(cfg)
	hr := NewHelmSyncer(locker.NewLocker())

	cacheName := "test-chart-dedup"

	// Pre-populate builtCharts to simulate another goroutine having already built
	result := &syncResult{done: make(chan struct{})}
	close(result.done) // already completed successfully
	hr.builtCharts.Store(cacheName, result)

	err := hr.buildChartOnce(app, cacheName, "/some/chart/dir")
	require.NoError(t, err)
	assert.Equal(t, int64(1), hr.BuildSkipped.Load())
	assert.Equal(t, int64(0), hr.BuildExecuted.Load())
}

func TestBuildChartOnceWithinRunDedupPropagatesError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &Config{
		ServiceDirName: ".myks",
		VendirCache:    "vendir-cache",
		RootDir:        tmpDir,
	}

	app := newDedupTestApp(cfg)
	hr := NewHelmSyncer(locker.NewLocker())

	cacheName := "test-chart-error"

	// Pre-populate with a failed result
	result := &syncResult{
		done: make(chan struct{}),
		err:  assert.AnError,
	}
	close(result.done)
	hr.builtCharts.Store(cacheName, result)

	err := hr.buildChartOnce(app, cacheName, "/some/chart/dir")
	assert.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, int64(1), hr.BuildSkipped.Load(), "should count as skipped even when error is propagated")
}

func TestBuildChartOnceFallbackKey(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &Config{
		ServiceDirName: ".myks",
		VendirCache:    "vendir-cache",
		RootDir:        tmpDir,
	}

	app := newDedupTestApp(cfg)
	hr := NewHelmSyncer(locker.NewLocker())

	chartDir := filepath.Join(tmpDir, "my-chart")

	// Pre-populate under the chartDir key (the fallback used when cacheName is empty)
	result := &syncResult{done: make(chan struct{})}
	close(result.done)
	hr.builtCharts.Store(chartDir, result)

	// Empty cacheName triggers the fallback key path
	err := hr.buildChartOnce(app, "", chartDir)
	require.NoError(t, err)
	assert.Equal(t, int64(1), hr.BuildSkipped.Load(), "fallback key dedup should count as skipped")
	assert.Equal(t, int64(0), hr.BuildExecuted.Load())
}

func TestBuildChartOnceConcurrentDedup(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &Config{
		ServiceDirName: ".myks",
		VendirCache:    "vendir-cache",
		RootDir:        tmpDir,
	}

	// Create a minimal Chart.yaml with no dependencies so helmBuild returns nil
	// without invoking any external binary.
	chartDir := filepath.Join(tmpDir, "test-chart")
	require.NoError(t, os.MkdirAll(chartDir, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(chartDir, "Chart.yaml"),
		[]byte("apiVersion: v2\nname: test-chart\nversion: 0.1.0\n"),
		0o600,
	))

	hr := NewHelmSyncer(locker.NewLocker())

	const numGoroutines = 10
	var wg sync.WaitGroup
	var errCount atomic.Int64

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app := newDedupTestApp(cfg)
			if err := hr.buildChartOnce(app, "concurrent-chart", chartDir); err != nil {
				errCount.Add(1)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(0), errCount.Load(), "no errors expected")
	assert.Equal(t, int64(1), hr.BuildExecuted.Load(), "exactly one goroutine should execute the build")
	assert.Equal(t, int64(numGoroutines-1), hr.BuildSkipped.Load(), "remaining goroutines should use in-run dedup")
}

func TestBuildChartOnceConcurrentDedupWithError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &Config{
		ServiceDirName: ".myks",
		VendirCache:    "vendir-cache",
		RootDir:        tmpDir,
	}

	// Use a nonexistent chartDir so helmBuild fails at the isExist check.
	chartDir := filepath.Join(tmpDir, "nonexistent-chart")

	hr := NewHelmSyncer(locker.NewLocker())

	const numGoroutines = 10
	var wg sync.WaitGroup
	var errCount atomic.Int64

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app := newDedupTestApp(cfg)
			if err := hr.buildChartOnce(app, "error-chart", chartDir); err != nil {
				errCount.Add(1)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(numGoroutines), errCount.Load(), "all goroutines should receive the error")
	assert.Equal(t, int64(1), hr.BuildExecuted.Load(), "exactly one goroutine should attempt the build")
	assert.Equal(t, int64(numGoroutines-1), hr.BuildSkipped.Load(), "remaining goroutines should use in-run dedup")
}
