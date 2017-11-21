package exec

import (
	"os"

	"github.com/concourse/atc/worker"
)

// OnSuccessStep will run one step, and then a second step if the first step
// succeeds.
type OnSuccessStep struct {
	stepFactory    StepFactory
	successFactory StepFactory

	repo *worker.ArtifactRepository

	step    Step
	success Step
}

// OnSuccess constructs an OnSuccessStep factory.
func OnSuccess(firstStep StepFactory, secondStep StepFactory) OnSuccessStep {
	return OnSuccessStep{
		stepFactory:    firstStep,
		successFactory: secondStep,
	}
}

// Using constructs an *OnSuccessStep.
func (o OnSuccessStep) Using(repo *worker.ArtifactRepository) Step {
	o.repo = repo

	o.step = o.stepFactory.Using(o.repo)
	return &o
}

// Run will call Run on the first step and wait for it to complete. If the
// first step errors, Run returns the error. OnSuccessStep is ready as soon as
// the first step is ready.
//
// If the first step succeeds (that is, its Success result is true), the second
// step is executed. If the second step errors, its error is returned.
func (o *OnSuccessStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	stepRunErr := o.step.Run(signals, ready)

	if stepRunErr != nil {
		return stepRunErr
	}

	success := o.step.Succeeded()

	if !success {
		return nil
	}

	o.success = o.successFactory.Using(o.repo)

	err := o.success.Run(signals, make(chan struct{}))
	return err
}

// Succeeded is true if the first step completed and the second
// step completed successfully.
func (o *OnSuccessStep) Succeeded() bool {
	return o.step.Succeeded() && o.success.Succeeded()
}
