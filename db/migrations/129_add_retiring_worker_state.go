package migrations

import "github.com/concourse/atc/db/migration"

func AddRetiringWorkerState(tx migration.LimitedTx) error {
	// Cannot delete the migration because then we would screw up the migration numbering
	return nil
}
