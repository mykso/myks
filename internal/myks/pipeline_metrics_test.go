package myks

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPipelineMetricsTrackAppDuration(t *testing.T) {
	t.Parallel()

	pm := NewPipelineMetrics(4)
	pm.TrackAppDuration(100 * time.Millisecond)
	pm.TrackAppDuration(200 * time.Millisecond)
	pm.Finish()

	assert.Equal(t, 300*time.Millisecond, pm.totalWorkTime)
	assert.Equal(t, 2, pm.appCount)
}

func TestPipelineMetricsWallClock(t *testing.T) {
	t.Parallel()

	pm := NewPipelineMetrics(2)
	time.Sleep(10 * time.Millisecond)
	pm.Finish()

	wall := pm.WallClock()
	assert.GreaterOrEqual(t, wall, 10*time.Millisecond)
}

func TestPipelineMetricsFinishIdempotent(t *testing.T) {
	t.Parallel()

	pm := NewPipelineMetrics(1)
	pm.Finish()
	first := pm.WallClock()

	time.Sleep(10 * time.Millisecond)
	pm.Finish() // second call should not update wallEnd

	second := pm.WallClock()
	assert.Equal(t, first, second, "Finish should be idempotent")
}

func TestPipelineMetricsBuildSummaryContainsFields(t *testing.T) {
	t.Parallel()

	pm := NewPipelineMetrics(8)
	pm.TrackAppDuration(500 * time.Millisecond)
	pm.TrackAppDuration(700 * time.Millisecond)
	pm.Finish()

	summary := pm.buildPipelineSummary()
	assert.Contains(t, summary, "Pipeline Concurrency Summary")
	assert.Contains(t, summary, "Apps processed:      2")
	assert.Contains(t, summary, "Async level:         8")
	assert.Contains(t, summary, "Wall clock time:")
	assert.Contains(t, summary, "Sequential estimate:")
	assert.Contains(t, summary, "Speedup:")
	assert.Contains(t, summary, "Efficiency:")
}

func TestPipelineMetricsBuildSummaryUnlimited(t *testing.T) {
	t.Parallel()

	pm := NewPipelineMetrics(-1)
	pm.Finish()

	summary := pm.buildPipelineSummary()
	assert.Contains(t, summary, "unlimited")
}

func TestPipelineMetricsBuildSummaryNoApps(t *testing.T) {
	t.Parallel()

	pm := NewPipelineMetrics(4)
	pm.Finish()

	summary := pm.buildPipelineSummary()
	// Should still produce a summary (wall clock + config), just without speedup fields.
	assert.Contains(t, summary, "Pipeline Concurrency Summary")
	assert.NotContains(t, summary, "Speedup:")
}
