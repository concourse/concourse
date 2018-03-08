package exec

import (
	"context"

	"github.com/concourse/atc/worker"
)

// Retry constructs a Step that will run the steps in order until one of them
// succeeds.
type Retry []StepFactory

// Using constructs a *RetryStep.
func (stepFactory Retry) Using(repo *worker.ArtifactRepository) Step {
	retry := &RetryStep{}

	for _, subStepFactory := range stepFactory {
		retry.Attempts = append(retry.Attempts, subStepFactory.Using(repo))
	}

	return retry
}

// RetryStep is a step that will run the steps in order until one of them
// succeeds.
type RetryStep struct {
	Attempts    []Step
	LastAttempt Step
}

// Run iterates through each step, stopping once a step succeeds. If all steps
// fail, the RetryStep will fail.
func (step *RetryStep) Run(ctx context.Context) error {
	var attemptErr error

	for _, attempt := range step.Attempts {
		step.LastAttempt = attempt

		attemptErr = attempt.Run(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if attemptErr != nil {
			continue
		}

		if attempt.Succeeded() {
			break
		}
	}

	return attemptErr
}

// Succeeded delegates to the last step that it ran.
func (step *RetryStep) Succeeded() bool {
	return step.LastAttempt.Succeeded()
}
