package exec

import "os"

type onSuccess struct {
	stepFactory    StepFactory
	successFactory StepFactory

	prev Step
	repo *SourceRepository

	step    Step
	success Step
}

func OnSuccess(
	stepFactory StepFactory,
	successFactory StepFactory,
) StepFactory {
	return onSuccess{
		stepFactory:    stepFactory,
		successFactory: successFactory,
	}
}

func (o onSuccess) Using(prev Step, repo *SourceRepository) Step {
	o.repo = repo
	o.prev = prev

	o.step = o.stepFactory.Using(o.prev, o.repo)
	return &o
}

func (o *onSuccess) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	stepRunErr := o.step.Run(signals, ready)

	if stepRunErr != nil {
		return stepRunErr
	}

	var success Success

	_ = o.step.Result(&success)

	if !success {
		return nil
	}

	o.success = o.successFactory.Using(o.step, o.repo)
	err := o.success.Run(signals, make(chan struct{}))
	return err
}

func (o *onSuccess) Result(x interface{}) bool {
	switch v := x.(type) {
	case *Success:
		if o.success == nil {
			// no second step means we must have failed the first step
			*v = false
			return true
		}
		stepResult := o.step.Result(v)
		if !stepResult {
			return false
		}
		//TODO: reset value of x when we cannot compare in the successStep.Result
		successResult := o.success.Result(v)
		if !successResult {
			return false
		}
		return true

	default:
		return false
	}
}

func (o *onSuccess) Release() {
	if o.step != nil {
		o.step.Release()
	}
	if o.success != nil {
		o.success.Release()
	}
}
