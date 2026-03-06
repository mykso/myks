package myks

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type StepMetric struct {
	TotalTime time.Duration
	UserTime  time.Duration
	SysTime   time.Duration
	MaxRSS    int64
	Count     int
}

type MetricsManager struct {
	mu      sync.Mutex
	metrics map[string]*StepMetric
}

func NewMetricsManager() *MetricsManager {
	return &MetricsManager{
		metrics: make(map[string]*StepMetric),
	}
}

var PrintStats bool

func (m *MetricsManager) TrackCmdMetric(step string, cmd *exec.Cmd, elapsed time.Duration) {
	if m == nil || step == "" || cmd == nil || cmd.ProcessState == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	metric, ok := m.metrics[step]
	if !ok {
		metric = &StepMetric{}
		m.metrics[step] = metric
	}

	metric.Count++
	metric.TotalTime += elapsed
	metric.UserTime += cmd.ProcessState.UserTime()
	metric.SysTime += cmd.ProcessState.SystemTime()

	rss := getCmdMaxRSS(cmd)
	if rss > metric.MaxRSS {
		metric.MaxRSS = rss
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

func (m *MetricsManager) PrintCmdMetrics() {
	if m == nil {
		return
	}

	m.mu.Lock()
	if len(m.metrics) == 0 {
		m.mu.Unlock()
		return
	}
	// Snapshot metrics under the lock so we can release it before formatting/logging.
	snapshot := make(map[string]*StepMetric, len(m.metrics))
	for k, v := range m.metrics {
		cp := *v
		snapshot[k] = &cp
	}
	m.mu.Unlock()

	summary := buildMetricsSummary(snapshot)
	if summary == "" {
		return
	}

	if PrintStats {
		fmt.Print(summary)
	} else {
		log.Info().Msg(summary)
	}
}
