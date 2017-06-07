package exec

import (
	"os"

	"github.com/concourse/atc/worker"
)

// Identity constructs a step that just propagates the previous step to the
// next one, without running anything.
type Identity struct{}

// Using constructs an IdentityStep.
func (Identity) Using(repo *worker.ArtifactRepository) Step {
	return IdentityStep{}
}

// IdentityStep does nothing, and delegates everything else to its nested step.
type IdentityStep struct {
	Step
}

// Run does nothing.
func (IdentityStep) Run(<-chan os.Signal, chan<- struct{}) error {
	return nil
}
