package migrations

import "github.com/concourse/atc/db/migration"

func RemoveResourceCheckingFromJobsAndAddManualyTriggeredToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
			ALTER TABLE jobs DROP COLUMN resource_checking;
		`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
			ALTER TABLE jobs DROP COLUMN resource_check_waiver_end;
		`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
			ALTER TABLE builds ADD COLUMN manually_triggered bool DEFAULT false;
		`)
	if err != nil {
		return err
	}

	return nil

}
