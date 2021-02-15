package exec

import (
	"context"
)

// RetryStep is a step that will run the steps in order until one of them
// succeeds.
type RetryStep struct {
	Attempts    []Step
	LastAttempt Step
}

func Retry(attempts ...Step) Step {
	return &RetryStep{
		Attempts: attempts,
	}
}

// Run iterates through each step, stopping once a step succeeds. If all steps
// fail, the RetryStep will fail.
func (step *RetryStep) Run(ctx context.Context, state RunState) (bool, error) {
	var attemptOk bool
	var attemptErr error

	for _, attempt := range step.Attempts {
		step.LastAttempt = attempt

		attemptOk, attemptErr = attempt.Run(ctx, state)
		if ctx.Err() != nil {
			return false, ctx.Err()
		}

		if attemptErr != nil {
			continue
		}

		if attemptOk {
			break
		}
	}

	return attemptOk, attemptErr
}
