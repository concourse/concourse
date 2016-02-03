package migrations

import "github.com/BurntSushi/migration"

func RemoveVolumesWithExpiredWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	DELETE FROM volumes v
	WHERE (SELECT COUNT(name) FROM workers w WHERE w.name = v.worker_name) = 0;
	`)
	return err
}
