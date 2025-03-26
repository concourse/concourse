package metric

import "sync/atomic"

type Counter struct {
	cur atomic.Int64
}

func (m *Counter) Inc() {
	m.cur.Add(1)
}

func (m *Counter) IncDelta(delta int) {
	m.cur.Add(int64(delta))
}

func (m *Counter) Delta() float64 {
	cur := m.cur.Swap(0)
	return float64(cur)
}
