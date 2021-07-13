package component

import "context"

// Runnable represents a workload to execute.
type Runnable interface {
	// Run is invoked repeatedly. The component should perform whatever work is
	// available and return.
	Run(context.Context) error
}

// RunFunc turns a simple function into a Runnable.
type RunFunc func(context.Context) error

func (f RunFunc) Run(ctx context.Context) error {
	return f(ctx)
}
