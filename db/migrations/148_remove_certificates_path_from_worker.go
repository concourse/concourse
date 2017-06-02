package migrations

import "github.com/concourse/atc/db/migration"

func RemoveCertificatesPathToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		DROP COLUMN certificates_path,
		DROP COLUMN certificates_symlinked_paths;
`)
	return err
}
