package exec

import (
	"context"
	"time"

	"github.com/concourse/atc/worker"
)

// TimeoutStep applies a fixed timeout to a step's Run.
type TimeoutStep struct {
	step     StepFactory
	runStep  Step
	duration string
	timedOut bool
}

// Timeout constructs a TimeoutStep factory.
func Timeout(
	step StepFactory,
	duration string,
) TimeoutStep {
	return TimeoutStep{
		step:     step,
		duration: duration,
		timedOut: false,
	}
}

// Using constructs a *TimeoutStep.
func (ts TimeoutStep) Using(repo *worker.ArtifactRepository) Step {
	ts.runStep = ts.step.Using(repo)

	return &ts
}

// Run parses the timeout duration and invokes the nested step.
//
// If the nested step takes longer than the duration, it is sent the Interrupt
// signal, and the TimeoutStep returns nil once the nested step exits (ignoring
// the nested step's error).
//
// The result of the nested step's Run is returned.
func (ts *TimeoutStep) Run(ctx context.Context) error {
	parsedDuration, err := time.ParseDuration(ts.duration)
	if err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, parsedDuration)
	defer cancel()

	err = ts.runStep.Run(timeoutCtx)
	if err == context.DeadlineExceeded {
		ts.timedOut = true
		return nil
	}

	return err
}

// Succeeded is true if the nested step completed successfully
// and did not time out.
func (ts *TimeoutStep) Succeeded() bool {
	return !ts.timedOut && ts.runStep.Succeeded()
}
