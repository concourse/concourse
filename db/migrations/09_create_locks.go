package migrations

import "github.com/concourse/atc/db/migration"

func CreateLocks(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE TABLE resource_checking_lock ()`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE build_scheduling_lock ()`)
	if err != nil {
		return err
	}

	return nil
}
