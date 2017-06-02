package migrations

import "github.com/concourse/atc/db/migration"

func AddImageResourceVersions(tx migration.LimitedTx) error {
	_, err := tx.Exec(`CREATE TABLE image_resource_versions (
    id serial PRIMARY KEY,
    version text NOT NULL,
    build_id integer REFERENCES builds (id) ON DELETE CASCADE NOT NULL,
    plan_id text NOT NULL,
    resource_hash text NOT NULL,
		UNIQUE (build_id, plan_id)
	)`)
	return err
}
