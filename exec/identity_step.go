package exec

import (
	"os"

	"github.com/concourse/atc/worker"
)

// Identity constructs an IdentityStep
type Identity struct{}

// Using constructs an IdentityStep.
func (Identity) Using(repo *worker.ArtifactRepository) Step {
	return IdentityStep{}
}

// IdentityStep does nothing
type IdentityStep struct {
}

// Run does nothing.
func (IdentityStep) Run(<-chan os.Signal, chan<- struct{}) error {
	return nil
}

func (IdentityStep) Succeeded() bool {
	return true
}
