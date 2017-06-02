package migrations

import "github.com/concourse/atc/db/migration"

func AddHijackURLToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE builds ADD COLUMN hijack_url varchar(255)`)
	if err != nil {
		return err
	}

	return nil
}
