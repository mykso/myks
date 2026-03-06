package myks

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// resetMetrics clears the global metrics map for test isolation.
func resetMetrics() {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	metrics = make(map[string]*StepMetric)
}

// runTrueCmd runs a no-op command and returns the *exec.Cmd with a valid ProcessState.
func runTrueCmd(t *testing.T) *exec.Cmd {
	t.Helper()
	// Run the test binary itself with a non-matching pattern so it exits
	// immediately – works portably on all platforms (no Unix-only "true").
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to run no-op cmd: %v", err)
	}
	return cmd
}

func TestTrackCmdMetric_GuardConditions(t *testing.T) {
	tests := []struct {
		name string
		step string
		cmd  func() *exec.Cmd
	}{
		{
			name: "empty step name",
			step: "",
			cmd:  func() *exec.Cmd { return runTrueCmd(t) },
		},
		{
			name: "nil cmd",
			step: "helm",
			cmd:  func() *exec.Cmd { return nil },
		},
		{
			name: "cmd without ProcessState",
			step: "helm",
			cmd:  func() *exec.Cmd { return exec.Command(os.Args[0], "-test.run=^$") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetMetrics()
			// None of these should panic or add a metric entry.
			TrackCmdMetric(tt.step, tt.cmd(), 100*time.Millisecond)

			metricsMu.Lock()
			count := len(metrics)
			metricsMu.Unlock()

			if count != 0 {
				t.Errorf("expected no metrics to be recorded, got %d", count)
			}
		})
	}
}

func TestTrackCmdMetric_Aggregation(t *testing.T) {
	resetMetrics()

	cmd1 := runTrueCmd(t)
	cmd2 := runTrueCmd(t)

	TrackCmdMetric("helm", cmd1, 100*time.Millisecond)
	TrackCmdMetric("helm", cmd2, 200*time.Millisecond)
	TrackCmdMetric("ytt", cmd1, 50*time.Millisecond)

	metricsMu.Lock()
	helmMetric, helmOk := metrics["helm"]
	yttMetric, yttOk := metrics["ytt"]
	metricsMu.Unlock()

	if !helmOk {
		t.Fatal("expected helm metric to be recorded")
	}
	if helmMetric.Count != 2 {
		t.Errorf("helm Count: got %d, want 2", helmMetric.Count)
	}
	if helmMetric.TotalTime != 300*time.Millisecond {
		t.Errorf("helm TotalTime: got %v, want 300ms", helmMetric.TotalTime)
	}

	if !yttOk {
		t.Fatal("expected ytt metric to be recorded")
	}
	if yttMetric.Count != 1 {
		t.Errorf("ytt Count: got %d, want 1", yttMetric.Count)
	}
	if yttMetric.TotalTime != 50*time.Millisecond {
		t.Errorf("ytt TotalTime: got %v, want 50ms", yttMetric.TotalTime)
	}
}

func TestTrackCmdMetric_MaxRSS(t *testing.T) {
	resetMetrics()

	cmd := runTrueCmd(t)

	// Manually inject MaxRSS to test the max-tracking logic without relying on
	// platform-specific RSS values from the real command.
	TrackCmdMetric("helm", cmd, 100*time.Millisecond)

	metricsMu.Lock()
	metrics["helm"].MaxRSS = 1024
	metricsMu.Unlock()

	TrackCmdMetric("helm", cmd, 100*time.Millisecond)

	metricsMu.Lock()
	rss := metrics["helm"].MaxRSS
	metricsMu.Unlock()

	// MaxRSS should remain >= 1024 since we set it explicitly above and the
	// second call will only overwrite if the new RSS is larger.
	if rss < 1024 {
		t.Errorf("MaxRSS should be >= 1024, got %d", rss)
	}
}

func TestBuildMetricsSummary_Empty(t *testing.T) {
	summary := buildMetricsSummary(make(map[string]*StepMetric))
	if summary != "" {
		t.Errorf("expected empty summary for empty metrics, got %q", summary)
	}
}

func TestBuildMetricsSummary_Content(t *testing.T) {
	m := map[string]*StepMetric{
		"helm": {Count: 3, TotalTime: 300 * time.Millisecond, UserTime: 150 * time.Millisecond, SysTime: 50 * time.Millisecond, MaxRSS: 2 * 1024 * 1024},
		"ytt":  {Count: 1, TotalTime: 100 * time.Millisecond, UserTime: 80 * time.Millisecond, SysTime: 10 * time.Millisecond, MaxRSS: 1024 * 1024},
	}

	summary := buildMetricsSummary(m)

	requiredSubstrings := []string{
		"--- Tool Resource Metrics Summary ---",
		"Step",
		"Count",
		"Total Time",
		"helm",
		"3",
		"300ms",
		"ytt",
		"1",
		"100ms",
		"2.00 MB",
		"1.00 MB",
	}

	for _, s := range requiredSubstrings {
		if !strings.Contains(summary, s) {
			t.Errorf("summary missing expected substring %q\nfull summary:\n%s", s, summary)
		}
	}
}

func TestBuildMetricsSummary_SortedSteps(t *testing.T) {
	m := map[string]*StepMetric{
		"zzz":  {Count: 1},
		"aaa":  {Count: 2},
		"mmm":  {Count: 3},
	}

	summary := buildMetricsSummary(m)

	aaaIdx := strings.Index(summary, "aaa")
	mmmIdx := strings.Index(summary, "mmm")
	zzzIdx := strings.Index(summary, "zzz")

	if aaaIdx == -1 || mmmIdx == -1 || zzzIdx == -1 {
		t.Fatal("expected all step names in summary")
	}
	if !(aaaIdx < mmmIdx && mmmIdx < zzzIdx) {
		t.Errorf("steps not in sorted order: aaa=%d, mmm=%d, zzz=%d", aaaIdx, mmmIdx, zzzIdx)
	}
}

func TestPrintCmdMetrics_PrintStatsFlag(t *testing.T) {
	resetMetrics()

	metricsMu.Lock()
	metrics["helm"] = &StepMetric{Count: 1, TotalTime: 100 * time.Millisecond}
	metricsMu.Unlock()

	// Capture zerolog output to verify the log path is used when PrintStats=false.
	var logBuf bytes.Buffer
	origLogger := log.Logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: &logBuf, NoColor: true})
	defer func() { log.Logger = origLogger }()

	origPrintStats := PrintStats
	defer func() { PrintStats = origPrintStats }()

	PrintStats = false
	PrintCmdMetrics()

	if !strings.Contains(logBuf.String(), "helm") {
		t.Errorf("expected helm in log output when PrintStats=false, got: %s", logBuf.String())
	}
}

func TestPrintCmdMetrics_EmptyMetrics(t *testing.T) {
	resetMetrics()
	// Should not panic and should produce no output.
	PrintCmdMetrics()
}
