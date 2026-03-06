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

var (
	PrintStats bool

	metricsMu sync.Mutex
	metrics   = make(map[string]*StepMetric)
)

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
	sb.WriteString(fmt.Sprintf("%-20s | %-6s | %-12s | %-12s | %-12s | %-10s\n",
		"Step", "Count", "Total Time", "User CPU", "System CPU", "Max Memory"))
	sb.WriteString(strings.Repeat("-", 85) + "\n")

	for _, step := range steps {
		sm := m[step]
		maxMemoryMB := float64(sm.MaxRSS) / 1024.0 / 1024.0 // Assuming RSS is in bytes
		sb.WriteString(fmt.Sprintf("%-20s | %-6d | %-12s | %-12s | %-12s | %.2f MB\n",
			step,
			sm.Count,
			sm.TotalTime.Round(time.Millisecond).String(),
			sm.UserTime.Round(time.Millisecond).String(),
			sm.SysTime.Round(time.Millisecond).String(),
			maxMemoryMB,
		))
	}
	sb.WriteString(strings.Repeat("-", 85) + "\n")

	return sb.String()
}

func PrintCmdMetrics() {
	metricsMu.Lock()
	defer metricsMu.Unlock()

	summary := buildMetricsSummary(metrics)
	if summary == "" {
		return
	}

	if PrintStats {
		fmt.Print(summary)
	} else {
		log.Info().Msg(summary)
	}
}
