// Package locker provides a per-item RWMutex registry for coordinating
// concurrent read and write access to items across goroutines.
package locker

import (
	"iter"
	"sort"
	"sync"
)

// Locker manages a per-item RWMutex registry.
type Locker struct {
	mu    sync.Mutex
	locks map[string]*sync.RWMutex
}

// LockReq describes what kind of access is needed for an item.
type LockReq struct {
	Name     string
	ForWrite bool
}

// NewLocker creates a new Locker with an empty lock registry.
func NewLocker() *Locker {
	return &Locker{locks: make(map[string]*sync.RWMutex)}
}

// getOrCreate lazily creates an RWMutex for a name.
func (l *Locker) getOrCreate(name string) *sync.RWMutex {
	l.mu.Lock()
	defer l.mu.Unlock()
	if lock, ok := l.locks[name]; ok {
		return lock
	}
	lock := &sync.RWMutex{}
	l.locks[name] = lock
	return lock
}

// Acquire locks multiple items in a consistent (sorted) order.
// Returns an unlock function — always defer it.
func (l *Locker) Acquire(reqs []LockReq) (unlock func()) {
	// sort by name to prevent deadlocks
	sorted := make([]LockReq, len(reqs))
	copy(sorted, reqs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	// Deduplicate: if same item requested for both read & write, escalate to write
	deduped := dedup(sorted)

	// Acquire in sorted order
	for _, req := range deduped {
		lk := l.getOrCreate(req.Name)
		if req.ForWrite {
			lk.Lock()
		} else {
			lk.RLock()
		}
	}

	// Release in reverse order (conventional, not strictly required)
	return func() {
		for i := len(deduped) - 1; i >= 0; i-- {
			lk := l.getOrCreate(deduped[i].Name)
			if deduped[i].ForWrite {
				lk.Unlock()
			} else {
				lk.RUnlock()
			}
		}
	}
}

// dedup merges duplicate names, escalating to write if any request is ForWrite.
func dedup(sorted []LockReq) []LockReq {
	if len(sorted) == 0 {
		return nil
	}
	result := []LockReq{sorted[0]}
	for i := 1; i < len(sorted); i++ {
		last := &result[len(result)-1]
		if sorted[i].Name == last.Name {
			last.ForWrite = last.ForWrite || sorted[i].ForWrite // escalate
		} else {
			result = append(result, sorted[i])
		}
	}
	return result
}

// LockNames is a convenience wrapper around Acquire for a list of names with the same access type.
func (l *Locker) LockNames(names iter.Seq[string], forWrite bool) func() {
	lockRequests := []LockReq{}
	for name := range names {
		lockRequests = append(lockRequests, LockReq{
			Name:     name,
			ForWrite: forWrite,
		})
	}
	return l.Acquire(lockRequests)
}
