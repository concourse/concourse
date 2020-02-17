package exec

import (
	"context"
	"time"

	"github.com/concourse/concourse/atc/worker"
)

// InterruptStep applies a timeout for aborts/interrupts of a step's Run.
type InterruptStep struct {
	step     Step
	duration string
}

// Interrupt constructs a InterruptStep factory.
func Interrupt(step Step, duration string) *InterruptStep {
	return &InterruptStep{
		step:     step,
		duration: duration,
	}
}

// Run parses the interrupt timeout duration and invokes the nested step.
//
//
// The result of the nested step's Run is returned.
func (is *InterruptStep) Run(ctx context.Context, state RunState) error {
	parsedDuration, err := time.ParseDuration(is.duration)
	if err != nil {
		return err
	}

	interruptCtx := worker.WithInterruptTimeout(ctx, parsedDuration)

	err = is.step.Run(interruptCtx, state)

	return err
}

// Succeeded is true if the nested step completed successfully.
func (is *InterruptStep) Succeeded() bool {
	return is.step.Succeeded()
}
