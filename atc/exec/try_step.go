package exec

import (
	"context"
)

// TryStep wraps another step, ignores its errors, and always succeeds.
type TryStep struct {
	step Step
	aborted bool
}

// Try constructs a TryStep.
func Try(step Step) Step {
	return &TryStep{
		step: step,
		aborted: false,
	}
}

// Run runs the nested step, and always returns nil, ignoring the nested step's
// error.
func (ts *TryStep) Run(ctx context.Context, state RunState) error {
	err := ts.step.Run(ctx, state)
	if err == context.Canceled {
		ts.aborted = true
		// propagate aborts but not timeouts
		return err
	}

	return nil
}

// Succeeded is true
func (ts *TryStep) Succeeded() bool {
	return !ts.aborted
}
