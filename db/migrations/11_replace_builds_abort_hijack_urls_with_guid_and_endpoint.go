package migrations

import "github.com/BurntSushi/migration"

func ReplaceBuildsAbortHijackURLsWithGuidAndEndpoint(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE builds DROP COLUMN abort_url`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE builds DROP COLUMN hijack_url`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE builds ADD COLUMN guid varchar(36)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE builds ADD COLUMN endpoint varchar(128)`)
	if err != nil {
		return err
	}

	return nil
}
