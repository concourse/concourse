package exec

import "os"

type try struct {
	step    StepFactory
	runStep Step
}

func Try(
	step StepFactory,
) StepFactory {
	return try{
		step: step,
	}
}

func (ts try) Using(prev Step, repo *SourceRepository) Step {
	ts.runStep = ts.step.Using(prev, repo)

	return &ts
}

func (ts *try) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	ts.runStep.Run(signals, ready)
	return nil
}

func (ts *try) Release() {
	ts.runStep.Release()
}

func (ts *try) Result(x interface{}) bool {
	switch v := x.(type) {
	case *Success:
		*v = Success(true)
		return true
	default:
		return ts.runStep.Result(x)
	}
}
