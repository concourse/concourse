package wrappa

import (
	"errors"
	"sync"
)

type Pool interface {
	TryAcquire() bool
	Release() error
}

type pool struct {
	size  int
	state int
	mu    sync.Mutex
}

func (p *pool) TryAcquire() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == p.size {
		return false
	}
	p.state += 1
	return true
}

func (p *pool) Release() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == 0 {
		return errors.New("released more than held")
	}
	p.state -= 1
	return nil
}
