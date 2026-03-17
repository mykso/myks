package myks

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mykso/myks/internal/locker"
	"github.com/rs/zerolog/log"
)

// StepMetric holds aggregated timing and resource usage metrics for a rendering step.
type StepMetric struct {
	TotalTime time.Duration
	UserTime  time.Duration
	SysTime   time.Duration
	MaxRSS    int64
	Count     int
}

var (
	// PrintStats controls where command metrics are printed: to stdout when true, or via log.Info when false.
	PrintStats bool

	metricsMu sync.Mutex
	metrics   = make(map[string]*StepMetric)

	globalPipelineMetricsMu sync.Mutex
	globalPipelineMetrics   *PipelineMetrics

	globalLockerStatsMu sync.Mutex
	globalLockerStats   *locker.Stats
)

// StorePipelineMetrics stores the pipeline metrics for later printing.
func StorePipelineMetrics(pm *PipelineMetrics) {
	globalPipelineMetricsMu.Lock()
	defer globalPipelineMetricsMu.Unlock()
	globalPipelineMetrics = pm
}

// StoreLockerStats stores the locker stats for later printing.
func StoreLockerStats(s *locker.Stats) {
	globalLockerStatsMu.Lock()
	defer globalLockerStatsMu.Unlock()
	globalLockerStats = s
}

// TrackCmdMetric records timing and resource usage for a completed command step.
func TrackCmdMetric(step string, cmd *exec.Cmd, elapsed time.Duration) {
	if step == "" || cmd == nil || cmd.ProcessState == nil {
		return
	}

	metricsMu.Lock()
	defer metricsMu.Unlock()

	m, ok := metrics[step]
	if !ok {
		m = &StepMetric{}
		metrics[step] = m
	}

	m.Count++
	m.TotalTime += elapsed
	m.UserTime += cmd.ProcessState.UserTime()
	m.SysTime += cmd.ProcessState.SystemTime()

	rss := getCmdMaxRSS(cmd)
	if rss > m.MaxRSS {
		m.MaxRSS = rss
	}
}

func buildMetricsSummary(m map[string]*StepMetric) string {
	if len(m) == 0 {
		return ""
	}
	var steps []string
	for k := range m {
		steps = append(steps, k)
	}
	sort.Strings(steps)

	var sb strings.Builder
	sb.WriteString("\n--- Tool Resource Metrics Summary ---\n")
	fmt.Fprintf(&sb, "%-20s | %-6s | %-12s | %-12s | %-12s | %-10s\n",
		"Step", "Count", "Total Time", "User CPU", "System CPU", "Max Memory")
	sb.WriteString(strings.Repeat("-", 85) + "\n")

	for _, step := range steps {
		sm := m[step]
		maxMemoryMB := float64(sm.MaxRSS) / 1024.0 / 1024.0 // Assuming RSS is in bytes
		fmt.Fprintf(&sb, "%-20s | %-6d | %-12s | %-12s | %-12s | %.2f MB\n",
			step,
			sm.Count,
			sm.TotalTime.Round(time.Millisecond).String(),
			sm.UserTime.Round(time.Millisecond).String(),
			sm.SysTime.Round(time.Millisecond).String(),
			maxMemoryMB,
		)
	}
	sb.WriteString(strings.Repeat("-", 85) + "\n")

	return sb.String()
}

// PrintCmdMetrics prints a summary of all tracked command metrics, pipeline concurrency
// statistics, and lock contention data. Output goes to stdout when PrintStats is true,
// or to the log otherwise.
func PrintCmdMetrics() {
	var combined strings.Builder

	// --- Tool Resource Metrics ---
	metricsMu.Lock()
	snapshot := make(map[string]*StepMetric, len(metrics))
	for k, v := range metrics {
		cp := *v
		snapshot[k] = &cp
	}
	metricsMu.Unlock()

	if cmdSummary := buildMetricsSummary(snapshot); cmdSummary != "" {
		combined.WriteString(cmdSummary)
	}

	// --- Pipeline Concurrency Summary ---
	globalPipelineMetricsMu.Lock()
	pm := globalPipelineMetrics
	globalPipelineMetricsMu.Unlock()

	if pm != nil {
		combined.WriteString(pm.buildPipelineSummary())
	}

	// --- Lock Contention Summary ---
	globalLockerStatsMu.Lock()
	ls := globalLockerStats
	globalLockerStatsMu.Unlock()

	if ls != nil {
		var wallClock time.Duration
		if pm != nil {
			wallClock = pm.WallClock()
		}
		combined.WriteString(ls.BuildSummary(wallClock))
	}

	summary := combined.String()
	if summary == "" {
		return
	}

	if PrintStats {
		fmt.Print(summary)
	} else {
		log.Info().Msg(summary)
	}
}
