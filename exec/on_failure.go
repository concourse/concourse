package exec

import (
	"os"

	"github.com/concourse/atc/worker"
)

// OnFailureStep will run one step, and then a second step if the first step
// fails (but not errors).
type OnFailureStep struct {
	stepFactory    StepFactory
	failureFactory StepFactory

	repo *worker.ArtifactRepository

	step Step
}

// OnFailure constructs an OnFailureStep factory.
func OnFailure(firstStep StepFactory, secondStep StepFactory) OnFailureStep {
	return OnFailureStep{
		stepFactory:    firstStep,
		failureFactory: secondStep,
	}
}

// Using constructs an *OnFailureStep.
func (o OnFailureStep) Using(repo *worker.ArtifactRepository) Step {
	o.repo = repo

	o.step = o.stepFactory.Using(o.repo)
	return &o
}

// Run will call Run on the first step and wait for it to complete. If the
// first step errors, Run returns the error. OnFailureStep is ready as soon as
// the first step is ready.
//
// If the first step fails (that is, its Success result is false), the second
// step is executed. If the second step errors, its error is returned.
func (o *OnFailureStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := o.step.Run(signals, ready)

	if err != nil {
		return err
	}

	if !o.step.Succeeded() {
		return o.failureFactory.Using(o.repo).Run(signals, make(chan struct{}))
	}

	return nil
}

// Succeeded is true if the first step doesn't exist, or if it
// completed successfully.
func (o *OnFailureStep) Succeeded() bool {
	return o.step.Succeeded()
}
