package db

import "time"

//go:generate counterfeiter . Clock

type Clock interface {
	Now() time.Time
	Until(time.Time) time.Duration
}

type clock struct{}

func NewClock() clock {
	return clock{}
}

func (c *clock) Now() time.Time {
	return time.Now()
}

func (c *clock) Until(t time.Time) time.Duration {
	return time.Until(t)
}
