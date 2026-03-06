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
		cmd  func(t *testing.T) *exec.Cmd
	}{
		{
			name: "empty step name",
			step: "",
			cmd:  func(t *testing.T) *exec.Cmd { return runTrueCmd(t) },
		},
		{
			name: "nil cmd",
			step: "helm",
			cmd:  func(t *testing.T) *exec.Cmd { return nil },
		},
		{
			name: "cmd without ProcessState",
			step: "helm",
			cmd:  func(t *testing.T) *exec.Cmd { return exec.Command(os.Args[0], "-test.run=^$") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetMetrics()
			// None of these should panic or add a metric entry.
			TrackCmdMetric(tt.step, tt.cmd(t), 100*time.Millisecond)

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

	TrackCmdMetric("helm", cmd, 100*time.Millisecond)

	// Set MaxRSS to a very large sentinel value – larger than any realistic
	// process RSS – so the second TrackCmdMetric call (whose actual RSS will
	// be far smaller) must NOT overwrite it.
	const sentinelRSS = int64(1<<62) - 1
	metricsMu.Lock()
	metrics["helm"].MaxRSS = sentinelRSS
	metricsMu.Unlock()

	TrackCmdMetric("helm", cmd, 100*time.Millisecond)

	metricsMu.Lock()
	rss := metrics["helm"].MaxRSS
	metricsMu.Unlock()

	if rss != sentinelRSS {
		t.Errorf("MaxRSS decreased after second call: got %d, want %d", rss, sentinelRSS)
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

	// Check that global header labels are present.
	headerSubstrings := []string{"--- Tool Resource Metrics Summary ---", "Step", "Count", "Total Time"}
	for _, s := range headerSubstrings {
		if !strings.Contains(summary, s) {
			t.Errorf("summary missing header substring %q\nfull summary:\n%s", s, summary)
		}
	}

	// Check per-step rows with specific patterns to avoid false matches from
	// other numeric values in the summary (e.g. "300ms" or "1.00 MB").
	// The count column uses %-6d so "3" is rendered as "3      " – the pattern
	// "| 3 " uniquely identifies the count field in that row.
	type rowCheck struct {
		step   string
		checks []string
	}
	for _, rc := range []rowCheck{
		{"helm", []string{"| 3 ", "300ms", "2.00 MB"}},
		{"ytt", []string{"| 1 ", "100ms", "1.00 MB"}},
	} {
		var row string
		for _, line := range strings.Split(summary, "\n") {
			// The step column uses %-20s (left-aligned, right-padded), so
			// data rows start directly with the step name – no leading whitespace.
			if strings.HasPrefix(line, rc.step) {
				row = line
				break
			}
		}
		if row == "" {
			t.Errorf("row for step %q not found in summary:\n%s", rc.step, summary)
			continue
		}
		for _, s := range rc.checks {
			if !strings.Contains(row, s) {
				t.Errorf("row for step %q missing %q\nrow: %s", rc.step, s, row)
			}
		}
	}
}

func TestBuildMetricsSummary_SortedSteps(t *testing.T) {
	m := map[string]*StepMetric{
		"zzz": {Count: 1},
		"aaa": {Count: 2},
		"mmm": {Count: 3},
	}

	summary := buildMetricsSummary(m)

	aaaIdx := strings.Index(summary, "aaa")
	mmmIdx := strings.Index(summary, "mmm")
	zzzIdx := strings.Index(summary, "zzz")

	if aaaIdx == -1 || mmmIdx == -1 || zzzIdx == -1 {
		t.Fatal("expected all step names in summary")
	}
	if aaaIdx >= mmmIdx || mmmIdx >= zzzIdx {
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

func TestPrintCmdMetrics_PrintStatsToStdout(t *testing.T) {
	resetMetrics()

	metricsMu.Lock()
	metrics["helm"] = &StepMetric{Count: 1, TotalTime: 100 * time.Millisecond}
	metricsMu.Unlock()

	origPrintStats := PrintStats
	defer func() { PrintStats = origPrintStats }()
	PrintStats = true

	// Capture stdout to verify the summary is printed when PrintStats=true.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	PrintCmdMetrics()
	_ = w.Close()

	var out bytes.Buffer
	_, _ = out.ReadFrom(r)
	summary := out.String()

	if !strings.Contains(summary, "--- Tool Resource Metrics Summary ---") {
		t.Errorf("expected summary header in stdout when PrintStats=true, got: %s", summary)
	}
	if !strings.Contains(summary, "helm") {
		t.Errorf("expected step name in stdout when PrintStats=true, got: %s", summary)
	}
}

func TestPrintCmdMetrics_EmptyMetrics(t *testing.T) {
	resetMetrics()
	// Should not panic and should produce no output.
	PrintCmdMetrics()
}
