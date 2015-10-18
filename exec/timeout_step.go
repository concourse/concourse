package exec

import (
	"os"
	"time"

	"github.com/pivotal-golang/clock"
	"github.com/tedsuo/ifrit"
)

type timeout struct {
	step     StepFactory
	runStep  Step
	duration string
	clock    clock.Clock
	timedOut bool
}

func Timeout(
	step StepFactory,
	duration string,
	clock clock.Clock,
) StepFactory {
	return timeout{
		step:     step,
		duration: duration,
		clock:    clock,
	}
}

func (ts timeout) Using(prev Step, repo *SourceRepository) Step {
	ts.runStep = ts.step.Using(prev, repo)

	return &ts
}

func (ts *timeout) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	parsedDuration, err := time.ParseDuration(ts.duration)
	if err != nil {
		return err
	}

	timer := ts.clock.NewTimer(parsedDuration)

	runProcess := ifrit.Invoke(ts.runStep)

	close(ready)

	var runErr error
	var sig os.Signal

dance:
	for {
		select {
		case runErr = <-runProcess.Wait():
			break dance
		case <-timer.C():
			ts.timedOut = true
			runProcess.Signal(os.Interrupt)
		case sig = <-signals:
			runProcess.Signal(sig)
		}
	}

	if ts.timedOut {
		// swallow interrupted error
		return nil
	}

	if runErr != nil {
		return runErr
	}

	return nil
}

func (ts *timeout) Release() {
	ts.runStep.Release()
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
