package util

import (
	"sync/atomic"
)

type SequenceGenerator interface {
	Next() int
}

type seqGenerator struct {
	current atomic.Int64
}

func NewSequenceGenerator(start int) SequenceGenerator {
	gen := &seqGenerator{}
	gen.current.Store(int64(start - 1))
	return gen
}

func (g *seqGenerator) Next() int {
	return int(g.current.Add(1))
}
