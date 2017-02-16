package migrations

import "github.com/concourse/atc/dbng/migration"

func AddInterceptibleToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	  ALTER TABLE builds
		ADD COLUMN interceptible BOOLEAN DEFAULT TRUE;
	`)
	if err != nil {
		return err
	}

	return nil
}
