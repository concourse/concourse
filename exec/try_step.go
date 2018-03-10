package exec

import (
	"context"

	"github.com/concourse/atc/worker"
)

// TryStep wraps another step, ignores its errors, and always succeeds.
type TryStep struct {
	step Step
}

// Try constructs a TryStep.
func Try(step Step) Step {
	return TryStep{
		step: step,
	}
}

// Run runs the nested step, and always returns nil, ignoring the nested step's
// error.
func (ts TryStep) Run(ctx context.Context, repo *worker.ArtifactRepository) error {
	err := ts.step.Run(ctx, repo)
	if err == context.Canceled {
		// propagate aborts but not timeouts
		return err
	}

	return nil
}

// Succeeded is true
func (ts TryStep) Succeeded() bool {
	return true
}
