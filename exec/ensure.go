package exec

import (
	"os"

	"github.com/hashicorp/go-multierror"
)

type ensure struct {
	stepFactory   StepFactory
	ensureFactory StepFactory

	prev Step
	repo *SourceRepository

	step   Step
	ensure Step
}

func Ensure(
	stepFactory StepFactory,
	ensureFactory StepFactory,
) StepFactory {
	return ensure{
		stepFactory:   stepFactory,
		ensureFactory: ensureFactory,
	}
}

func (o ensure) Using(prev Step, repo *SourceRepository) Step {
	o.repo = repo
	o.prev = prev

	o.step = o.stepFactory.Using(o.prev, o.repo)
	return &o
}

func (o *ensure) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	var errors error

	originalErr := o.step.Run(signals, ready) // don't care about the error
	if originalErr != nil {
		errors = multierror.Append(errors, originalErr)
	}

	// The contract of the Result method is such that it does not change the value
	// of the provided pointer if it is not able to respond.
	// Therefore there is no need to check the return value here.
	o.ensure = o.ensureFactory.Using(o.step, o.repo)

	hookErr := o.ensure.Run(signals, make(chan struct{})) // TODO test
	if hookErr != nil {
		errors = multierror.Append(errors, hookErr)
	}

	return errors
}

func (o *ensure) Result(x interface{}) bool {
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
func (o *ensure) Release() {
	if o.step != nil {
		o.step.Release()
	}
	if o.ensure != nil {
		o.ensure.Release()
	}
}
