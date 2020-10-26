package batch

import (
	"context"
	"fmt"
)

type Migrator interface {
	Migrate(context.Context) (bool, error)
	Cleanup(context.Context) error
}

type Runner struct {
	Migrator Migrator
}

func (runner Runner) Run(ctx context.Context) error {
	cleanup, err := runner.Migrator.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	if cleanup {
		err = runner.Migrator.Cleanup(ctx)
		if err != nil {
			return fmt.Errorf("cleanup: %w", err)
		}
	}

	return nil
}
