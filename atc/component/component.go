package component

type Component interface {
	Name() string
	Paused() bool

	IntervalElapsed() bool

	Reload() (bool, error)
	UpdateLastRan() error
}

