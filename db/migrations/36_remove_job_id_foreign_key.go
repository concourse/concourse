package migrations

import "github.com/BurntSushi/migration"

func RemoveJobIDForeignKey(tx migration.LimitedTx) error {

	_, err := tx.Exec(`
		ALTER TABLE builds DROP CONSTRAINT fkey_job_id;
`)

	return err

}
