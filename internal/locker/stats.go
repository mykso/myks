package locker

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// LockStat holds aggregated acquisition and timing data for a single named lock.
type LockStat struct {
	AcquireCount  int
	ReadCount     int
	WriteCount    int
	TotalWaitTime time.Duration // cumulative time blocked waiting to acquire
	MaxWaitTime   time.Duration // worst single wait
	TotalHoldTime time.Duration // cumulative time lock held
	MaxHoldTime   time.Duration // worst single hold
}

// Stats aggregates LockStat entries for all named locks in a Locker.
// It is always-on; the overhead is negligible compared to the second-scale I/O the locks protect.
type Stats struct {
	mu    sync.Mutex
	stats map[string]*LockStat
}

func newStats() Stats {
	return Stats{stats: make(map[string]*LockStat)}
}

func (s *Stats) recordAcquire(name string, forWrite bool, waitDur time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, ok := s.stats[name]
	if !ok {
		st = &LockStat{}
		s.stats[name] = st
	}

	st.AcquireCount++
	if forWrite {
		st.WriteCount++
	} else {
		st.ReadCount++
	}
	st.TotalWaitTime += waitDur
	if waitDur > st.MaxWaitTime {
		st.MaxWaitTime = waitDur
	}
}

func (s *Stats) recordRelease(name string, holdDur time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, ok := s.stats[name]
	if !ok {
		return
	}

	st.TotalHoldTime += holdDur
	if holdDur > st.MaxHoldTime {
		st.MaxHoldTime = holdDur
	}
}

// Snapshot returns a safe copy of current stats keyed by lock name.
func (s *Stats) Snapshot() map[string]LockStat {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make(map[string]LockStat, len(s.stats))
	for k, v := range s.stats {
		result[k] = *v
	}
	return result
}

// BuildSummary formats the top-10 locks by total wait time.
// wallClock is used to compute the percentage of wall time spent waiting; pass 0 to skip.
func (s *Stats) BuildSummary(wallClock time.Duration) string {
	snap := s.Snapshot()
	if len(snap) == 0 {
		return ""
	}

	names := make([]string, 0, len(snap))
	for k := range snap {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool {
		return snap[names[i]].TotalWaitTime > snap[names[j]].TotalWaitTime
	})

	const maxRows = 10
	if len(names) > maxRows {
		names = names[:maxRows]
	}

	var totalWait time.Duration
	for _, st := range snap {
		totalWait += st.TotalWaitTime
	}

	const colW = 95
	var sb strings.Builder
	sb.WriteString("\n--- Lock Contention Summary ---\n")
	fmt.Fprintf(&sb, "%-24s | %-6s | %-8s | %-12s | %-10s | %-12s | %-10s\n",
		"Lock Name", "Acq", "R/W", "Total Wait", "Max Wait", "Total Hold", "Max Hold")
	sb.WriteString(strings.Repeat("-", colW) + "\n")

	for _, name := range names {
		st := snap[name]
		fmt.Fprintf(&sb, "%-24s | %-6d | %-8s | %-12s | %-10s | %-12s | %-10s\n",
			truncateLockName(name, 24),
			st.AcquireCount,
			fmt.Sprintf("%d/%d", st.ReadCount, st.WriteCount),
			st.TotalWaitTime.Round(time.Millisecond).String(),
			st.MaxWaitTime.Round(time.Millisecond).String(),
			st.TotalHoldTime.Round(time.Millisecond).String(),
			st.MaxHoldTime.Round(time.Millisecond).String(),
		)
	}
	sb.WriteString(strings.Repeat("-", colW) + "\n")

	if wallClock > 0 {
		pct := float64(totalWait) / float64(wallClock) * 100
		fmt.Fprintf(&sb, "Total lock wait: %s (%.1f%% of wall clock)\n",
			totalWait.Round(time.Millisecond).String(), pct)
	} else {
		fmt.Fprintf(&sb, "Total lock wait: %s\n", totalWait.Round(time.Millisecond).String())
	}

	return sb.String()
}

func truncateLockName(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
