package component

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Component
type Component interface {
	Name() string
	Paused() bool

	IntervalElapsed() bool

	Reload() (bool, error)
	UpdateLastRan() error
}
