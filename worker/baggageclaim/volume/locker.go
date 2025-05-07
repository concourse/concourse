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
	refCount atomic.Int32
}

func NewLockManager() LockManager {
	return &lockManager{}
}

func (m *lockManager) Lock(key string) {
	var entry *lockEntry

	if value, ok := m.locks.Load(key); ok {
		entry = value.(*lockEntry)
		entry.refCount.Add(1)
	} else {
		newEntry := &lockEntry{}
		newEntry.refCount.Add(1)
		actual, loaded := m.locks.LoadOrStore(key, newEntry)
		if loaded {
			entry = actual.(*lockEntry)
			entry.refCount.Add(1)
		} else {
			entry = newEntry
		}
	}

	// Now actually acquire the lock for the caller
	entry.mu.Lock()
}

func (m *lockManager) Unlock(key string) {
	value, ok := m.locks.Load(key)
	if !ok {
		panic(fmt.Sprintf("key %q already unlocked", key))
	}
	entry := value.(*lockEntry)

	// Release the actual lock
	entry.mu.Unlock()

	newCount := entry.refCount.Add(-1)
	if newCount == 0 {
		m.locks.Delete(key)
	}
}
