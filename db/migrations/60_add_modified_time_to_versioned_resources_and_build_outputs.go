package migrations

import "github.com/concourse/atc/db/migration"

func AddModifiedTimeToVersionedResourcesAndBuildOutputs(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE versioned_resources
		ADD COLUMN modified_time timestamp NOT NULL DEFAULT now();
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE build_outputs
		ADD COLUMN modified_time timestamp NOT NULL DEFAULT now();
`)
	return err
}
