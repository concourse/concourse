package migrations

import "github.com/BurntSushi/migration"

func DropOldLocks(tx migration.LimitedTx) error {
	_, err := tx.Exec(`DROP TABLE resource_checking_lock`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DROP TABLE build_scheduling_lock`)
	if err != nil {
		return err
	}

	return nil
}
