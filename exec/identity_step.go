package exec

import (
	"context"

	"github.com/concourse/atc/worker"
)

// IdentityStep does nothing.
type IdentityStep struct{}

// Run does nothing.
func (IdentityStep) Run(context.Context, *worker.ArtifactRepository) error {
	return nil
}

func (IdentityStep) Succeeded() bool {
	return true
}
