package wrappa

import (
	"golang.org/x/sync/semaphore"
)

type Pool interface {
	TryAcquire() bool
	Release()
}

type pool struct {
	*semaphore.Weighted
}

func NewPool(size int) Pool {
	return &pool{
		semaphore.NewWeighted(int64(size)),
	}
}

func (p *pool) TryAcquire() bool {
	return p.Weighted.TryAcquire(1)
}

func (p *pool) Release() {
	p.Weighted.Release(1)
}
