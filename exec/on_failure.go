package exec

import "os"

type onFailure struct {
	stepFactory    StepFactory
	failureFactory StepFactory

	prev Step
	repo *SourceRepository

	step    Step
	failure Step
}

func OnFailure(
	stepFactory StepFactory,
	failureFactory StepFactory,
) StepFactory {
	return onFailure{
		stepFactory:    stepFactory,
		failureFactory: failureFactory,
	}
}

func (o onFailure) Using(prev Step, repo *SourceRepository) Step {
	o.repo = repo
	o.prev = prev

	o.step = o.stepFactory.Using(o.prev, o.repo)
	return &o
}

func (o *onFailure) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
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

func (o *onFailure) Result(x interface{}) bool {
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

func (o *onFailure) Release() {
	if o.step != nil {
		o.step.Release()
	}
	if o.failure != nil {
		o.failure.Release()
	}
}
