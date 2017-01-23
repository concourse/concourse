package exec

import (
	"os"

	"github.com/concourse/atc/worker"
	"github.com/hashicorp/go-multierror"
)

// EnsureStep will run one step, and then a second step regardless of whether
// the first step fails or errors.
type EnsureStep struct {
	stepFactory   StepFactory
	ensureFactory StepFactory

	prev Step
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
func (o EnsureStep) Using(prev Step, repo *worker.ArtifactRepository) Step {
	o.repo = repo
	o.prev = prev

	o.step = o.stepFactory.Using(o.prev, o.repo)
	return &o
}

// Run will call Run on the first step, wait for it to complete, and then call
// Run on the second step, regardless of whether the first step failed or
// errored.
//
// If the first step or the second step errors, an aggregate of their errors is
// returned.
func (o *EnsureStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	var errors error

	originalErr := o.step.Run(signals, ready)
	if originalErr != nil {
		errors = multierror.Append(errors, originalErr)
	}

	o.ensure = o.ensureFactory.Using(o.step, o.repo)

	hookErr := o.ensure.Run(signals, make(chan struct{}))
	if hookErr != nil {
		errors = multierror.Append(errors, hookErr)
	}

	return errors
}

// Result indicates Success by and-ing together both step's Success results. If
// either of them cannot indicate Success, it returns false.
//
// All other result types are ignored.
func (o *EnsureStep) Result(x interface{}) bool {
	switch v := x.(type) {
	case *Success:
		var aSuccess Success
		stepResult := o.step.Result(&aSuccess)
		if !stepResult {
			return false
		}

		var bSuccess Success
		ensureResult := o.ensure.Result(&bSuccess)
		if !ensureResult {
			return false
		}

		*v = aSuccess && bSuccess

		return true

	default:
		return false
	}
}
