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

	prev Step
	repo *worker.ArtifactRepository

	step    Step
	failure Step
}

// OnFailure constructs an OnFailureStep factory.
func OnFailure(firstStep StepFactory, secondStep StepFactory) OnFailureStep {
	return OnFailureStep{
		stepFactory:    firstStep,
		failureFactory: secondStep,
	}
}

// Using constructs an *OnFailureStep.
func (o OnFailureStep) Using(prev Step, repo *worker.ArtifactRepository) Step {
	o.repo = repo
	o.prev = prev

	o.step = o.stepFactory.Using(o.prev, o.repo)
	return &o
}

// Run will call Run on the first step and wait for it to complete. If the
// first step errors, Run returns the error. OnFailureStep is ready as soon as
// the first step is ready.
//
// If the first step fails (that is, its Success result is false), the second
// step is executed. If the second step errors, its error is returned.
func (o *OnFailureStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	stepRunErr := o.step.Run(signals, ready)

	if stepRunErr != nil {
		return stepRunErr
	}

	var success Success

	// The contract of the Result method is such that it does not change the value
	// of the provided pointer if it is not able to respond.
	// Therefore there is no need to check the return value here.
	_ = o.step.Result(&success)

	if !success {
		o.failure = o.failureFactory.Using(o.step, o.repo)
		err := o.failure.Run(signals, make(chan struct{}))
		return err
	}

	return nil
}

// Result indicates Success as true if the first step completed successfully.
//
// Any other type is ignored.
func (o *OnFailureStep) Result(x interface{}) bool {
	switch v := x.(type) {
	case *Success:
		if o.failure == nil {
			*v = true
			return true
		}
		*v = false
		return true

	default:
		return false
	}
}
