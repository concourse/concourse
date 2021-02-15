package exec

import (
	"context"
)

// IdentityStep does nothing.
type IdentityStep struct{}

// Run does nothing... successfully.
func (IdentityStep) Run(context.Context, RunState) (bool, error) {
	return true, nil
}
