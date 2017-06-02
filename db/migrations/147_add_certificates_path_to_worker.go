package migrations

import "github.com/concourse/atc/db/migration"

func AddCertificatesPathToWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE workers
		ADD COLUMN certificates_path text,
		ADD COLUMN certificates_symlinked_paths json NOT NULL DEFAULT 'null';
`)
	return err
}
