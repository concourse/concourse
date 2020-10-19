package exec

import (
	"context"
	"errors"
)

// TryStep wraps another step, ignores its errors, and always succeeds.
type TryStep struct {
	step    Step
	aborted bool
}

// Try constructs a TryStep.
func Try(step Step) Step {
	return &TryStep{
		step:    step,
		aborted: false,
	}
}

// Run runs the nested step, and always returns nil, ignoring the nested step's
// error.
func (ts *TryStep) Run(ctx context.Context, state RunState) (bool, error) {
	_, err := ts.step.Run(ctx, state)
	if errors.Is(err, context.Canceled) {
		// propagate aborts errors, but not timeouts
		return false, err
	}

	return true, nil
}
