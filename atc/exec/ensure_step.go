package exec

import (
	"context"

	"github.com/hashicorp/go-multierror"
)

// EnsureStep will run one step, and then a second step regardless of whether
// the first step fails or errors.
type EnsureStep struct {
	step Step
	hook Step
}

// Ensure constructs an EnsureStep.
func Ensure(step Step, hook Step) EnsureStep {
	return EnsureStep{
		step: step,
		hook: hook,
	}
}

// Run will call Run on the first step, wait for it to complete, and then call
// Run on the second step, regardless of whether the first step failed or
// errored.
//
// If the first step or the second step errors, an aggregate of their errors is
// returned.
func (o EnsureStep) Run(ctx context.Context, state RunState) (bool, error) {
	var errors error

	originalOk, originalErr := o.step.Run(ctx, state)
	if originalErr != nil {
		errors = multierror.Append(errors, originalErr)
	}

	hookCtx := ctx
	if ctx.Err() != nil {
		// prevent hook from being immediately canceled
		hookCtx = context.Background()
	}

	hookOk, hookErr := o.hook.Run(hookCtx, state)
	if hookErr != nil {
		errors = multierror.Append(errors, hookErr)
	}

	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	if errors != nil {
		return false, errors
	}

	return originalOk && hookOk, nil
}
