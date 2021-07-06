package component

import "context"

// RunResult represents some data returned a previous run and passed to the next run.
type RunResult interface {
	String() string
}

// Runnable represents a workload to execute.
type Runnable interface {
	// Run is invoked repeatedly. The component should perform whatever work is
	// available and return. Run can return a RunResult, and returned RunResult
	// will be stringified and pass to the next Run. If a component doesn't needs
	// to use RunResult, then just return nil for RunResult.
	Run(context.Context, string) (RunResult, error)
}

// RunFunc turns a simple function into a Runnable.
type RunFunc func(context.Context, string) (RunResult, error)

func (f RunFunc) Run(ctx context.Context, lastRunResult string) (RunResult, error) {
	return f(ctx, lastRunResult)
}
