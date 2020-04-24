package lock

import "time"

func NewMutex(timeout time.Duration) *mutex {
	return &mutex{
		c:  make(chan struct{}, 1),
		to: timeout,
	}
}

type mutex struct {
	c  chan struct{}
	to time.Duration
}

func (m *mutex) Unlock() {
	<-m.c
}

func (m *mutex) Lock() bool {
	return m.LockWithTimeout(m.to)
}

func (m *mutex) LockWithTimeout(timeout time.Duration) bool {
	if timeout > 0 {
		return m.lockWithTimeout(timeout)
	} else {
		return m.lockIndefinitely()
	}
}

func (m *mutex) lockWithTimeout(timeout time.Duration) bool {
	select {
	case m.c <- struct{}{}:
		return true
	case <-time.After(timeout):
	}
	return false
}

func (m *mutex) lockIndefinitely() bool {
	m.c <- struct{}{}
	return true
}
