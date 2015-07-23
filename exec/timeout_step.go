package exec

import (
	"errors"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/tedsuo/ifrit"
)

type timeout struct {
	step     StepFactory
	runStep  Step
	duration atc.Duration
	timedOut bool
}

var ErrStepTimedOut = errors.New("process-exceeded-timeout-limit")

func Timeout(
	step StepFactory,
	duration atc.Duration,
) StepFactory {
	return timeout{
		step:     step,
		duration: duration,
	}
}

func (ts timeout) Using(prev Step, repo *SourceRepository) Step {
	ts.runStep = ts.step.Using(prev, repo)

	return &ts
}

func (ts *timeout) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	runProcess := ifrit.Invoke(ts.runStep)

	timer := time.NewTimer(time.Duration(ts.duration))

	var runErr error
	var timeoutErr error
	var sig os.Signal

dance:
	for {
		select {
		case runErr = <-runProcess.Wait():
			break dance
		case <-timer.C:
			ts.timedOut = true
			timeoutErr = ErrStepTimedOut
			runProcess.Signal(os.Kill)
		case sig = <-signals:
			runProcess.Signal(sig)
		}
	}

	if timeoutErr != nil {
		return timeoutErr
	}

	if runErr != nil {
		return runErr
	}

	return nil
}

func (ts *timeout) Release() error {
	return ts.runStep.Release()
}

func (ts *timeout) Result(x interface{}) bool {
	switch v := x.(type) {
	case *Success:
		var success Success
		ts.runStep.Result(&success)
		*v = success && !Success(ts.timedOut)
		return true
	}
	return false
}
