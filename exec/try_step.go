package exec

import (
	"os"

	"github.com/concourse/atc/worker"
)

// TryStep wraps another step, ignores its errors, and always succeeds.
type TryStep struct {
	step    StepFactory
	runStep Step
}

// Try constructs a TryStep factory.
func Try(step StepFactory) TryStep {
	return TryStep{
		step: step,
	}
}

// Using constructs a *TryStep.
func (ts TryStep) Using(repo *worker.ArtifactRepository) Step {
	ts.runStep = ts.step.Using(repo)
	return &ts
}

// Run runs the nested step, and always returns nil, ignoring the nested step's
// error.
func (ts *TryStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := ts.runStep.Run(signals, ready)
	if err == ErrInterrupted {
		return err
	}
	return nil
}

// Succeeded is true
func (ts *TryStep) Succeeded() bool {
	return true
}
