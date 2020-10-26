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
	_, err := runner.Migrator.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// XXX(6203): we should enable this someday - but not this day.
	//
	// it's not safe to run Cleanup because it breaks the down migration.
	//
	// the complexity of this may be a sign that it would make more sense for
	// incremental/batch migrations to be a feature of the migration library
	// itself so it can know whether it's safe to perform the down migration.

	// if cleanup {
	// 	err = runner.Migrator.Cleanup(ctx)
	// 	if err != nil {
	// 		return fmt.Errorf("cleanup: %w", err)
	// 	}
	// }

	return nil
}
