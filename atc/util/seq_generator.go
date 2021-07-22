package util

import "sync"

type SequenceGenerator interface {
	Next() int
}

type seqGenerator struct {
	current int
	lock sync.Mutex
}

func NewSequenceGenerator(start int) SequenceGenerator {
	return &seqGenerator{
		current: start,
	}
}

func (g *seqGenerator) Next() int {
	g.lock.Lock()
	defer g.lock.Unlock()

	next := g.current
	g.current ++
	return next
}
