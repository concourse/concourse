package migrations

import "github.com/concourse/atc/db/migration"

func RemoveVolumesWithExpiredWorkers(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
	DELETE FROM volumes v
	WHERE (SELECT COUNT(name) FROM workers w WHERE w.name = v.worker_name) = 0;
	`)
	return err
}
