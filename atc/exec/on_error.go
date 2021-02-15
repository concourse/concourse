package exec

import (
	"context"
	"errors"

	"github.com/hashicorp/go-multierror"
)

// OnErrorStep will run one step, and then a second step if the first step
// errors.
type OnErrorStep struct {
	step Step
	hook Step
}

// OnError constructs an OnErrorStep factory.
func OnError(step Step, hook Step) OnErrorStep {
	return OnErrorStep{
		step: step,
		hook: hook,
	}
}

// Run will call Run on the first step and wait for it to complete. If the
// first step errors, Run returns the error. OnErrorStep is ready as soon as
// the first step is ready.
//
// If the first step errors, the second
// step is executed. If the second step errors, nothing is returned.
func (o OnErrorStep) Run(ctx context.Context, state RunState) (bool, error) {
	var errs error
	stepRunOk, stepRunErr := o.step.Run(ctx, state)
	// with no error, we just want to return right away
	if stepRunErr == nil {
		return stepRunOk, nil
	}
	errs = multierror.Append(errs, stepRunErr)

	// for all errors that aren't caused by an Abort, run the hook
	if !errors.Is(stepRunErr, context.Canceled) {
		_, err := o.hook.Run(context.Background(), state)
		if err != nil {
			// This causes to return both the errors as expected.
			errs = multierror.Append(errs, err)
		}
	}

	return stepRunOk, errs
}
