package metric

import "sync/atomic"

type Counter struct {
	cur int64
	max int64
}

func (c *Counter) Inc() {
	cur := atomic.AddInt64(&c.cur, 1)

	for {
		max := atomic.LoadInt64(&c.max)
		if cur > max {
			if atomic.CompareAndSwapInt64(&c.max, max, cur) {
				break
			}
		} else {
			break
		}
	}
}

func (c *Counter) Dec() {
	atomic.AddInt64(&c.cur, -1)
}

func (c *Counter) Max() int {
	cur := atomic.LoadInt64(&c.cur)
	max := atomic.SwapInt64(&c.max, -1)

	if max == -1 {
		// no call to .Inc has occurred since last call to .Max;
		// highest value must be the current value
		return int(cur)
	}

	return int(max)
}
