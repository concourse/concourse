package worker

import (
	"sync"
	"sync/atomic"
)

// countingWaitGroup extends sync.WaitGroup with a count that can be queried.
// It maintains the count separately using atomic.Int64 for efficient reads.
type countingWaitGroup struct {
	sync.WaitGroup
	count atomic.Int64
	mu    sync.Mutex
}

// Add increments the WaitGroup counter and the atomic count tracker atomically.
// This ensures that the count is always in sync with the WaitGroup's internal state.
func (cwg *countingWaitGroup) Add(n int) {
	cwg.mu.Lock()
	cwg.WaitGroup.Add(n)
	cwg.count.Add(int64(n))
	cwg.mu.Unlock()
}

// Done decrements the WaitGroup counter and the atomic count tracker atomically.
// This ensures that the count is always in sync with the WaitGroup's internal state.
func (cwg *countingWaitGroup) Done() {
	cwg.mu.Lock()
	cwg.WaitGroup.Done()
	cwg.count.Add(-1)
	cwg.mu.Unlock()
}

// Count returns the current count without locking.
func (cwg *countingWaitGroup) Count() int {
	return int(cwg.count.Load())
}
