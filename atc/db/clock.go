package db

import "time"

//counterfeiter:generate . Clock
type Clock interface {
	Now() time.Time
	Until(time.Time) time.Duration
}

type realClock struct{}

func NewClock() realClock {
	return realClock{}
}

func (c *realClock) Now() time.Time {
	return time.Now()
}

func (c *realClock) Until(t time.Time) time.Duration {
	return time.Until(t)
}
