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
	o.updateGetStep(o.step)

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

// Set GetStep's flag "registerUponFailure".
func (o OnFailureStep) updateGetStep(step Step) {
	switch step.(type) {
	case AggregateStep:
		for _, s := range ([]Step)(step.(AggregateStep)) {
			o.updateGetStep(s)
		}
	case InParallelStep:
		for _, s := range step.(InParallelStep).steps {
			o.updateGetStep(s)
		}
	case LogErrorStep:
		// Get is actually wrapped in a LogErrorStep, so we need to handle
		// LogErrorStep.
		o.updateGetStep(step.(LogErrorStep).Step)
	case *GetStep:
		step.(*GetStep).SetRegisterUponFailure(true)
	case OnSuccessStep:
		// For Put's implied Get, they are wired by an OnSuccessStep, Put as
		// the first step, implied Get as the second step. That's reason why
		// we need to handle both "step" and "hook".
		o.updateGetStep(step.(OnSuccessStep).step)
		o.updateGetStep(step.(OnSuccessStep).hook)
	case *TimeoutStep:
		o.updateGetStep(step.(*TimeoutStep).step)
	case TryStep:
		o.updateGetStep(step.(TryStep).step)
	}
}
