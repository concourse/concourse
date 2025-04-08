package metric

import "sync/atomic"

type Gauge struct {
	cur atomic.Int64
	max atomic.Int64
}

func (c *Gauge) Inc() {
	cur := c.cur.Add(1)
	c.updateMaxIfGreater(cur)
}

func (c *Gauge) Set(val int64) {
	c.updateMaxIfGreater(val)
}

func (c *Gauge) Dec() {
	c.cur.Add(-1)
}

func (c *Gauge) Max() float64 {
	cur := c.cur.Load()
	max := c.max.Swap(-1)

	if max == -1 {
		// max is -1, indicating no maximum value has been recorded
		// since the last call to Max(); in this case, return the
		// current counter value as it's the best available gauge
		return float64(cur)
	}

	return float64(max)
}

// Helper method to update max value if the provided value is greater
func (c *Gauge) updateMaxIfGreater(val int64) {
	for {
		max := c.max.Load()
		if val <= max {
			return // No need to update
		}

		if c.max.CompareAndSwap(max, val) {
			return // Successfully updated
		}
		// If CompareAndSwap failed, loop and try again
	}
}
