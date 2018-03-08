package exec

import (
	"context"

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
func (IdentityStep) Run(context.Context) error {
	return nil
}

func (IdentityStep) Succeeded() bool {
	return true
}
