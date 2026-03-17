package myks

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// PipelineMetrics tracks wall-clock and per-app processing times for a Run() call.
type PipelineMetrics struct {
	mu            sync.Mutex
	wallStart     time.Time
	wallEnd       time.Time
	appCount      int
	asyncLevel    int
	totalWorkTime time.Duration // sum of per-app processing durations
}

// NewPipelineMetrics creates a PipelineMetrics with the wall clock started.
func NewPipelineMetrics(asyncLevel int) *PipelineMetrics {
	return &PipelineMetrics{
		wallStart:  time.Now(),
		asyncLevel: asyncLevel,
	}
}

// TrackAppDuration records the processing duration for a single app.
func (pm *PipelineMetrics) TrackAppDuration(d time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.appCount++
	pm.totalWorkTime += d
}

// Finish records the wall-clock end time.
func (pm *PipelineMetrics) Finish() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.wallEnd.IsZero() {
		pm.wallEnd = time.Now()
	}
}

// WallClock returns the elapsed wall-clock duration.
func (pm *PipelineMetrics) WallClock() time.Duration {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.wallEnd.IsZero() {
		return time.Since(pm.wallStart)
	}
	return pm.wallEnd.Sub(pm.wallStart)
}

func (pm *PipelineMetrics) buildPipelineSummary() string {
	pm.mu.Lock()
	appCount := pm.appCount
	asyncLevel := pm.asyncLevel
	totalWork := pm.totalWorkTime
	wallStart := pm.wallStart
	wallEnd := pm.wallEnd
	pm.mu.Unlock()

	var wall time.Duration
	if wallEnd.IsZero() {
		wall = time.Since(wallStart)
	} else {
		wall = wallEnd.Sub(wallStart)
	}

	var sb strings.Builder
	sb.WriteString("\n--- Pipeline Concurrency Summary ---\n")

	asyncDesc := fmt.Sprintf("%d", asyncLevel)
	if asyncLevel <= 0 {
		asyncDesc = "unlimited"
	}

	fmt.Fprintf(&sb, "Apps processed:      %d\n", appCount)
	fmt.Fprintf(&sb, "Async level:         %s\n", asyncDesc)
	fmt.Fprintf(&sb, "Wall clock time:     %s\n", wall.Round(time.Millisecond).String())

	if appCount > 0 && totalWork > 0 {
		fmt.Fprintf(&sb, "Sequential estimate: %s\n", totalWork.Round(time.Millisecond).String())

		if wall > 0 {
			speedup := float64(totalWork) / float64(wall)
			fmt.Fprintf(&sb, "Speedup:             %.2fx\n", speedup)

			effectiveParallelism := float64(asyncLevel)
			if asyncLevel <= 0 {
				effectiveParallelism = float64(appCount)
			}
			if effectiveParallelism > 1 {
				efficiency := speedup / effectiveParallelism * 100
				fmt.Fprintf(&sb, "Efficiency:          %.1f%%\n", efficiency)
			}
		}
	}

	return sb.String()
}
