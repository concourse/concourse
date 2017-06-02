package migrations

import "github.com/concourse/atc/db/migration"

func AddVolumesAndCacheInvalidator(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE TABLE volumes (
	  id serial primary key,
    worker_name text NOT NULL,
		expires_at timestamp NOT NULL,
		ttl text null,
		handle text not null,
		resource_version text not null,
		resource_hash text not null,
		UNIQUE (handle)
	)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE cache_invalidator (
		last_invalidated timestamp NOT NULL DEFAULT 'epoch'
	)`)
	if err != nil {
		return err
	}

	return nil
}
