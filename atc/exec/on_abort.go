package exec

import (
	"context"
)

// OnAbortStep will run one step, and then a second step if the first step
// aborts (but not errors).
type OnAbortStep struct {
	step Step
	hook Step
}

// OnAbort constructs an OnAbortStep factory.
func OnAbort(step Step, hook Step) OnAbortStep {
	return OnAbortStep{
		step: step,
		hook: hook,
	}
}

// Run will call Run on the first step and wait for it to complete. If the
// first step errors, Run returns the error. OnAbortStep is ready as soon as
// the first step is ready.
//
// If the first step aborts (that is, it gets interrupted), the second
// step is executed. If the second step errors, its error is returned.
func (o OnAbortStep) Run(ctx context.Context, state RunState) error {
	stepRunErr := o.step.Run(ctx, state)
	if stepRunErr == nil {
		return nil
	}

	if stepRunErr == context.Canceled {
		// run only on abort, not timeout
		o.hook.Run(context.Background(), state)
	}

	return stepRunErr
}

// Succeeded is true if the first step doesn't exist, or if it
// completed successfully.
func (o OnAbortStep) Succeeded() bool {
	return o.step.Succeeded()
}
