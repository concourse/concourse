package exec

import (
	"context"

	"github.com/concourse/atc/worker"
	"github.com/hashicorp/go-multierror"
)

// EnsureStep will run one step, and then a second step regardless of whether
// the first step fails or errors.
type EnsureStep struct {
	stepFactory   StepFactory
	ensureFactory StepFactory

	repo *worker.ArtifactRepository

	step   Step
	ensure Step
}

// Ensure constructs an EnsureStep factory.
func Ensure(firstStep StepFactory, secondStep StepFactory) EnsureStep {
	return EnsureStep{
		stepFactory:   firstStep,
		ensureFactory: secondStep,
	}
}

// Using constructs an *EnsureStep.
func (o EnsureStep) Using(repo *worker.ArtifactRepository) Step {
	o.repo = repo

	o.step = o.stepFactory.Using(o.repo)
	return &o
}

// Run will call Run on the first step, wait for it to complete, and then call
// Run on the second step, regardless of whether the first step failed or
// errored.
//
// If the first step or the second step errors, an aggregate of their errors is
// returned.
func (o *EnsureStep) Run(ctx context.Context) error {
	var errors error

	originalErr := o.step.Run(ctx)
	if originalErr != nil {
		errors = multierror.Append(errors, originalErr)
	}

	o.ensure = o.ensureFactory.Using(o.repo)

	hookCtx := ctx
	if ctx.Err() != nil {
		// prevent hook from being immediately canceled
		hookCtx = context.Background()
	}

	hookErr := o.ensure.Run(hookCtx)
	if hookErr != nil {
		errors = multierror.Append(errors, hookErr)
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	return errors
}

// Succeeded is true if both of its steps are true
func (o *EnsureStep) Succeeded() bool {
	return o.step.Succeeded() && o.ensure.Succeeded()
}
