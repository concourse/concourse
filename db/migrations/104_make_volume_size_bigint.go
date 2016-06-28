package migrations

import "github.com/BurntSushi/migration"

func MakeVolumeSizeBigint(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE volumes ALTER COLUMN size TYPE bigint;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes RENAME COLUMN size TO size_in_bytes;
	`)
	return err
}
