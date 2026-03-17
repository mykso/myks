package locker_test

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mykso/myks/internal/locker"
)

// TestAcquireSortsOrder verifies that Acquire acquires locks in a consistent
// sorted order regardless of the order in which they are requested. It does
// this by verifying that two goroutines requesting the same locks in different
// orders both succeed without deadlocking (if they ran in insertion order they
// would deadlock).
func TestAcquireSortsOrder(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()
	done := make(chan struct{}, 2)

	// Goroutine 1 requests "a" then "b"
	go func() {
		unlock := lk.Acquire([]locker.LockReq{
			{Name: "a", ForWrite: true},
			{Name: "b", ForWrite: true},
		})
		time.Sleep(10 * time.Millisecond)
		unlock()
		done <- struct{}{}
	}()

	// Goroutine 2 requests "b" then "a" — opposite order. Without sorting
	// this would deadlock; with sorting both are acquired as a→b.
	go func() {
		unlock := lk.Acquire([]locker.LockReq{
			{Name: "b", ForWrite: true},
			{Name: "a", ForWrite: true},
		})
		time.Sleep(10 * time.Millisecond)
		unlock()
		done <- struct{}{}
	}()

	timeout := time.After(5 * time.Second)
	for range 2 {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("deadlock detected: goroutines did not complete within timeout")
		}
	}
}

// TestAcquireDeduplicatesReadRead verifies that requesting the same name twice
// with read access is deduplicated to a single read lock.
func TestAcquireDeduplicatesReadRead(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()
	unlock := lk.Acquire([]locker.LockReq{
		{Name: "x", ForWrite: false},
		{Name: "x", ForWrite: false},
	})
	// Unlock must not panic (double-RUnlock would panic).
	unlock()
}

// TestAcquireDeduplicatesReadWriteEscalation verifies that requesting the same
// name for both read and write escalates to a write lock (a single Lock, not
// RLock+Lock which would deadlock).
func TestAcquireDeduplicatesReadWriteEscalation(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()

	// If escalation is broken (RLock+Lock), this would deadlock because a
	// goroutine cannot hold RLock and then acquire Lock on the same mutex.
	done := make(chan struct{}, 1)
	go func() {
		unlock := lk.Acquire([]locker.LockReq{
			{Name: "x", ForWrite: false},
			{Name: "x", ForWrite: true},
		})
		unlock()
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock detected: read+write escalation did not complete")
	}
}

// TestAcquireDeduplicatesWriteWrite verifies that requesting the same name
// twice with write access is deduplicated to a single write lock.
func TestAcquireDeduplicatesWriteWrite(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()

	done := make(chan struct{}, 1)
	go func() {
		unlock := lk.Acquire([]locker.LockReq{
			{Name: "x", ForWrite: true},
			{Name: "x", ForWrite: true},
		})
		unlock()
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock detected: write+write deduplication did not complete")
	}
}

// TestWriterBlocksReaders verifies that a writer holding a lock prevents
// concurrent readers on the same name from proceeding.
func TestWriterBlocksReaders(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()

	// Acquire a write lock on "shared".
	writeUnlock := lk.Acquire([]locker.LockReq{{Name: "shared", ForWrite: true}})

	// Launch a reader; it should block until the writer releases.
	readStarted := make(chan struct{})
	readDone := make(chan struct{})
	go func() {
		close(readStarted)
		unlock := lk.Acquire([]locker.LockReq{{Name: "shared", ForWrite: false}})
		unlock()
		close(readDone)
	}()

	<-readStarted
	// Give the reader time to block on the mutex.
	time.Sleep(50 * time.Millisecond)

	select {
	case <-readDone:
		t.Fatal("reader should be blocked while writer holds the lock")
	default:
	}

	// Release the writer; reader should now proceed.
	writeUnlock()

	select {
	case <-readDone:
	case <-time.After(5 * time.Second):
		t.Fatal("reader did not proceed after writer released the lock")
	}
}

// TestReadersBlockWriter verifies that concurrent readers prevent a writer from
// acquiring the lock.
func TestReadersBlockWriter(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()

	readUnlock := lk.Acquire([]locker.LockReq{{Name: "shared", ForWrite: false}})

	writerDone := make(chan struct{})
	go func() {
		unlock := lk.Acquire([]locker.LockReq{{Name: "shared", ForWrite: true}})
		unlock()
		close(writerDone)
	}()

	time.Sleep(50 * time.Millisecond)

	select {
	case <-writerDone:
		t.Fatal("writer should be blocked while reader holds the lock")
	default:
	}

	readUnlock()

	select {
	case <-writerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("writer did not proceed after reader released the lock")
	}
}

// TestIndependentNamesConcurrent verifies that locks on independent names do
// not block each other.
func TestIndependentNamesConcurrent(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()

	var mu sync.Mutex
	var order []int

	var wg sync.WaitGroup
	wg.Add(2)

	// Both goroutines acquire write locks on different names simultaneously.
	go func() {
		defer wg.Done()
		unlock := lk.Acquire([]locker.LockReq{{Name: "alpha", ForWrite: true}})
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		unlock()
	}()

	go func() {
		defer wg.Done()
		unlock := lk.Acquire([]locker.LockReq{{Name: "beta", ForWrite: true}})
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		unlock()
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("independent locks should not block each other")
	}

	// Both goroutines must have run concurrently (both recorded entries).
	mu.Lock()
	defer mu.Unlock()
	if !slices.Contains(order, 1) || !slices.Contains(order, 2) {
		t.Fatalf("expected both goroutines to run, got order=%v", order)
	}
}

// TestConcurrentReadersDontBlockEachOther verifies that multiple goroutines can
// hold read locks on the same name simultaneously.
func TestConcurrentReadersDontBlockEachOther(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()
	const n = 10

	var wg sync.WaitGroup
	wg.Add(n)

	// All goroutines acquire a read lock; they should all proceed concurrently
	// rather than serialising.
	start := time.Now()
	for range n {
		go func() {
			defer wg.Done()
			unlock := lk.Acquire([]locker.LockReq{{Name: "shared", ForWrite: false}})
			time.Sleep(30 * time.Millisecond)
			unlock()
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent readers timed out")
	}

	// All n readers ran concurrently: total time should be close to 30 ms, not
	// n×30 ms. Allow generous overhead (3× the sleep) for slow CI.
	elapsed := time.Since(start)
	if elapsed > 90*time.Millisecond {
		t.Fatalf("readers appear to have serialised: elapsed=%v", elapsed)
	}
}

// TestLockNamesForRead verifies the LockNames convenience wrapper for read access.
func TestLockNamesForRead(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()
	names := []string{"c", "a", "b"}
	unlock := lk.LockNames(slices.Values(names), false)
	// Just ensure it doesn't panic and unlocks cleanly.
	unlock()
}

// TestLockNamesForWrite verifies the LockNames convenience wrapper for write access.
func TestLockNamesForWrite(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()
	names := []string{"c", "a", "b"}
	unlock := lk.LockNames(slices.Values(names), true)
	unlock()
}

// TestAcquireEmpty verifies that Acquire with an empty slice is a no-op and
// returns a callable unlock function.
func TestAcquireEmpty(t *testing.T) {
	t.Parallel()

	lk := locker.NewLocker()
	unlock := lk.Acquire(nil)
	unlock()

	unlock2 := lk.Acquire([]locker.LockReq{})
	unlock2()
}
