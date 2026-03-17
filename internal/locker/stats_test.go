package locker_test

import (
	"sync"
	"testing"
	"time"

	"github.com/mykso/myks/internal/locker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatsPopulatedAfterAcquireUnlock verifies that acquire/release increments
// counts and records hold time.
func TestStatsPopulatedAfterAcquireUnlock(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()
	unlock := lk.Acquire([]locker.LockReq{{Name: "item", ForWrite: true}})
	time.Sleep(5 * time.Millisecond)
	unlock()

	snap := lk.GetStats().Snapshot()
	require.Contains(t, snap, "item")
	st := snap["item"]

	assert.Equal(t, 1, st.AcquireCount)
	assert.Equal(t, 0, st.ReadCount)
	assert.Equal(t, 1, st.WriteCount)
	assert.Positive(t, st.TotalHoldTime, "hold time should be recorded")
	assert.Positive(t, st.MaxHoldTime, "max hold time should be recorded")
}

// TestStatsReadCount verifies that read-lock acquisitions increment ReadCount, not WriteCount.
func TestStatsReadCount(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()
	unlock := lk.Acquire([]locker.LockReq{{Name: "item", ForWrite: false}})
	unlock()

	snap := lk.GetStats().Snapshot()
	require.Contains(t, snap, "item")
	st := snap["item"]

	assert.Equal(t, 1, st.ReadCount)
	assert.Equal(t, 0, st.WriteCount)
}

// TestStatsMultipleAcquires verifies that repeated acquisitions accumulate correctly.
func TestStatsMultipleAcquires(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()
	for range 3 {
		unlock := lk.Acquire([]locker.LockReq{{Name: "shared", ForWrite: true}})
		unlock()
	}

	snap := lk.GetStats().Snapshot()
	require.Contains(t, snap, "shared")
	assert.Equal(t, 3, snap["shared"].AcquireCount)
	assert.Equal(t, 3, snap["shared"].WriteCount)
}

// TestStatsContentionProducesWaitTime verifies that a goroutine waiting for a
// held write lock accumulates non-zero wait time.
func TestStatsContentionProducesWaitTime(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()

	// Hold the write lock for a measurable duration.
	writeUnlock := lk.Acquire([]locker.LockReq{{Name: "contended", ForWrite: true}})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// This will block until writeUnlock() is called.
		unlock := lk.Acquire([]locker.LockReq{{Name: "contended", ForWrite: false}})
		unlock()
	}()

	time.Sleep(30 * time.Millisecond)
	writeUnlock()
	wg.Wait()

	snap := lk.GetStats().Snapshot()
	require.Contains(t, snap, "contended")
	st := snap["contended"]

	// The reader goroutine waited at least ~30 ms.
	assert.GreaterOrEqual(t, st.TotalWaitTime, 20*time.Millisecond,
		"contended lock should record non-zero wait time")
	assert.Positive(t, st.MaxWaitTime)
}

// TestStatsSnapshotIsSafeCopy verifies that mutating the snapshot does not
// affect the internal stats.
func TestStatsSnapshotIsSafeCopy(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()
	unlock := lk.Acquire([]locker.LockReq{{Name: "x", ForWrite: true}})
	unlock()

	snap := lk.GetStats().Snapshot()
	snap["x"] = locker.LockStat{AcquireCount: 9999}

	// Internal state should be unaffected.
	snap2 := lk.GetStats().Snapshot()
	assert.Equal(t, 1, snap2["x"].AcquireCount)
}

// TestStatsBuildSummaryEmpty verifies that an empty Stats returns an empty summary.
func TestStatsBuildSummaryEmpty(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()
	summary := lk.GetStats().BuildSummary(0)
	assert.Empty(t, summary)
}

// TestStatsBuildSummaryContainsLockName verifies the formatted summary includes
// acquired lock names.
func TestStatsBuildSummaryContainsLockName(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()
	unlock := lk.Acquire([]locker.LockReq{{Name: "my-cache-entry", ForWrite: true}})
	unlock()

	summary := lk.GetStats().BuildSummary(1 * time.Second)
	assert.Contains(t, summary, "my-cache-entry")
	assert.Contains(t, summary, "Lock Contention Summary")
	assert.Contains(t, summary, "Total lock wait")
}
