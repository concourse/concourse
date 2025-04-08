package volume

import (
	"fmt"
	"sync"
	"sync/atomic"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate. LockManager
type LockManager interface {
	Lock(key string)
	Unlock(key string)
}

type lockManager struct {
	locks sync.Map // key -> *lockEntry
}

type lockEntry struct {
	mu       sync.Mutex
	refCount int32 // Using int32 for atomic operations
}

func NewLockManager() LockManager {
	return &lockManager{}
}

func (m *lockManager) Lock(key string) {
	// Get or create a lock entry for this key
	var entry *lockEntry

	// First, try to load existing entry
	if value, ok := m.locks.Load(key); ok {
		entry = value.(*lockEntry)
		// Increment reference count atomically
		atomic.AddInt32(&entry.refCount, 1)
	} else {
		// Create new entry with refCount = 1
		newEntry := &lockEntry{refCount: 1}
		actual, loaded := m.locks.LoadOrStore(key, newEntry)
		if loaded {
			// Someone else created it first, use that one and increment
			entry = actual.(*lockEntry)
			atomic.AddInt32(&entry.refCount, 1)
		} else {
			entry = newEntry
		}
	}

	// Now actually acquire the lock for the caller
	entry.mu.Lock()
}

func (m *lockManager) Unlock(key string) {
	// Get the lock entry
	value, ok := m.locks.Load(key)
	if !ok {
		panic(fmt.Sprintf("key %q already unlocked", key))
	}
	entry := value.(*lockEntry)

	// Release the actual lock
	entry.mu.Unlock()

	// Decrement reference count and clean up if needed
	newCount := atomic.AddInt32(&entry.refCount, -1)
	if newCount == 0 {
		// Safe to delete as no one else is using it
		m.locks.Delete(key)
	}
}
