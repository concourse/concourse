package wrappa

import (
	"golang.org/x/sync/semaphore"
)

//go:generate counterfeiter . Pool

type Pool interface {
	Size() int
	TryAcquire() bool
	Release()
}

type pool struct {
	*semaphore.Weighted
	size int
}

func NewPool(size int) Pool {
	return &pool{
		Weighted: semaphore.NewWeighted(int64(size)),
		size:     size,
	}
}

func (p *pool) Size() int {
	return p.size
}

func (p *pool) TryAcquire() bool {
	return p.Weighted.TryAcquire(1)
}

func (p *pool) Release() {
	p.Weighted.Release(1)
}
