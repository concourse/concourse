package migrations

import "github.com/concourse/atc/dbng/migration"

func AddRetiringWorkerState(tx migration.LimitedTx) error {
	// Cannot delete the migration because then we would screw up the migration numbering
	return nil
}
