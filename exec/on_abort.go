package exec

import (
	"context"

	"github.com/concourse/atc/worker"
)

// OnAbortStep will run one step, and then a second step if the first step
// aborts (but not errors).
type OnAbortStep struct {
	stepFactory  StepFactory
	abortFactory StepFactory

	repo *worker.ArtifactRepository

	step Step
}

// OnAbort constructs an OnAbortStep factory.
func OnAbort(firstStep StepFactory, secondStep StepFactory) OnAbortStep {
	return OnAbortStep{
		stepFactory:  firstStep,
		abortFactory: secondStep,
	}
}

// Using constructs an *OnAbortStep.
func (o OnAbortStep) Using(repo *worker.ArtifactRepository) Step {
	o.repo = repo

	o.step = o.stepFactory.Using(o.repo)
	return &o
}

// Run will call Run on the first step and wait for it to complete. If the
// first step errors, Run returns the error. OnAbortStep is ready as soon as
// the first step is ready.
//
// If the first step aborts (that is, it gets interrupted), the second
// step is executed. If the second step errors, its error is returned.
func (o *OnAbortStep) Run(ctx context.Context) error {
	stepRunErr := o.step.Run(ctx)
	if stepRunErr == nil {
		return nil
	}

	if stepRunErr == context.Canceled {
		// run only on abort, not timeout
		o.abortFactory.Using(o.repo).Run(context.Background())
	}

	return stepRunErr
}

// Succeeded is true if the first step doesn't exist, or if it
// completed successfully.
func (o *OnAbortStep) Succeeded() bool {
	return o.step.Succeeded()
}
