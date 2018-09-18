package exec

import (
	"context"
)

// OnFailureStep will run one step, and then a second step if the first step
// fails (but not errors).
type OnFailureStep struct {
	step Step
	hook Step
}

// OnFailure constructs an OnFailureStep factory.
func OnFailure(firstStep Step, secondStep Step) OnFailureStep {
	return OnFailureStep{
		step: firstStep,
		hook: secondStep,
	}
}

// Run will call Run on the first step and wait for it to complete. If the
// first step errors, Run returns the error. OnFailureStep is ready as soon as
// the first step is ready.
//
// If the first step fails (that is, its Success result is false), the second
// step is executed. If the second step errors, its error is returned.
func (o OnFailureStep) Run(ctx context.Context, state RunState) error {
	err := o.step.Run(ctx, state)
	if err != nil {
		return err
	}

	if !o.step.Succeeded() {
		return o.hook.Run(ctx, state)
	}

	return nil
}

// Succeeded is true if the first step doesn't exist, or if it
// completed successfully.
func (o OnFailureStep) Succeeded() bool {
	return o.step.Succeeded()
}
