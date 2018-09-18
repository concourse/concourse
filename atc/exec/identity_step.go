package exec

import (
	"context"
)

// IdentityStep does nothing.
type IdentityStep struct{}

// Run does nothing.
func (IdentityStep) Run(context.Context, RunState) error {
	return nil
}

func (IdentityStep) Succeeded() bool {
	return true
}
