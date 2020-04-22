package metric

import "sync/atomic"

type Counter struct {
	cur int64
}

func (m *Counter) Inc() {
	atomic.AddInt64((*int64)(&m.cur), 1)
}

func (m *Counter) IncDelta(delta int) {
	atomic.AddInt64((*int64)(&m.cur), int64(delta))
}

func (m *Counter) Delta() int {
	cur := atomic.SwapInt64((*int64)(&m.cur), 0)
	return int(cur)
}
