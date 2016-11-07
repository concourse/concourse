package migrations

import "github.com/BurntSushi/migration"

func RemoveTTLFromVolumes(tx migration.LimitedTx) error {
	var err error

	_, err = tx.Exec(`
		ALTER TABLE volumes
		DROP COLUMN ttl,
		DROP COLUMN expires_at;
	`)
	if err != nil {
		return err
	}

	return nil
}
