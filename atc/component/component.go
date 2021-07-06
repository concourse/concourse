package component

import "time"

type Component interface {
	Name() string
	Paused() bool

	IntervalElapsed() bool

	Reload() (bool, error)
	UpdateLastRan(when time.Time, result RunResult) error

	LastRunResult() string
}
