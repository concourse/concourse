package lock

import (
	"sync"

	"code.cloudfoundry.org/lager"
)

func NewInMemoryFactory() LockFactory {
	return &inMemoryFactory{
		locks: map[string]*inMemoryLock{},
	}
}

type inMemoryFactory struct {
	mtx   sync.Mutex
	locks map[string]*inMemoryLock
}

func (i *inMemoryFactory) Acquire(_ lager.Logger, id LockID) (Lock, bool, error) {
	i.mtx.Lock()
	defer i.mtx.Unlock()

	key := id.toKey()

	lock, ok := i.locks[key]
	if !ok {
		lock = &inMemoryLock{
			lock: make(chan struct{}, 1),
		}
		i.locks[key] = lock
	}

	select {
	case lock.lock <- struct{}{}:
		return lock, true, nil
	default:
		return nil, false, nil
	}
}

type inMemoryLock struct {
	lock chan struct{}
}

func (i *inMemoryLock) Release() error {
	<-i.lock
	return nil
}
