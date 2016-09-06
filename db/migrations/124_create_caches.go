package migrations

import "github.com/BurntSushi/migration"

func CreateCaches(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TYPE volume_state AS ENUM (
			'creating',
			'created',
			'initializing',
			'initialized',
			'destroying'
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TYPE container_state AS ENUM (
			'creating',
			'created',
			'destroying'
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes
		ALTER COLUMN handle DROP NOT NULL,
		ADD COLUMN state volume_state NOT NULL DEFAULT 'creating',
		ADD COLUMN parent_id int,
		ADD COLUMN parent_state volume_state,
		ADD UNIQUE (id, state),
		ADD FOREIGN KEY (parent_id, parent_state) REFERENCES volumes (id, state) ON DELETE RESTRICT,
		ADD CONSTRAINT handle_when_created CHECK (
			(state = 'creating' AND handle IS NULL) OR (state != 'creating')
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ALTER COLUMN handle DROP NOT NULL,
		ADD COLUMN state container_state NOT NULL DEFAULT 'creating',
		ADD FOREIGN KEY (build_id) REFERENCES builds (id) ON DELETE SET NULL,
		ADD FOREIGN KEY (resource_id) REFERENCES resources (id) ON DELETE SET NULL,
		ADD COLUMN resource_type_id int REFERENCES resource_types (id) ON DELETE SET NULL,
		ADD COLUMN hijacked bool NOT NULL DEFAULT false,
		ADD CONSTRAINT handle_when_created CHECK (
			(state = 'creating' AND handle IS NULL) OR (state != 'creating')
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE caches (
			id serial PRIMARY KEY,
			resource_type_volume_id int NOT NULL,
			resource_type_volume_state volume_state NOT NULL,
			source_hash text NOT NULL,
			params_hash text NOT NULL,
			version TEXT NOT NULL,
			UNIQUE (resource_type_volume_id, source_hash, params_hash, version),
			FOREIGN KEY (resource_type_volume_id, resource_type_volume_state) REFERENCES volumes (id, state) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE worker_resource_types (
			id serial PRIMARY KEY,
			worker_name text REFERENCES workers (name) ON DELETE CASCADE,
			type text NOT NULL,
			image text NOT NULL,
			version text NOT NULL,
			UNIQUE (worker_name, type, image, version)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes
		ADD COLUMN cache_id int REFERENCES caches (id) ON DELETE SET NULL,
		ADD COLUMN worker_resource_type_id int REFERENCES worker_resource_types (id) ON DELETE SET NULL,
		ADD CONSTRAINT cannot_invalidate_during_initialization CHECK (
			(
				state = 'initialized' AND (
					(
						cache_id IS NULL
					) AND (
						worker_resource_type_id IS NULL
					) AND (
						container_id IS NULL
					)
				)
			) OR (
				(
					cache_id IS NOT NULL
				) OR (
					worker_resource_type_id IS NOT NULL
				) OR (
					container_id IS NOT NULL
				)
			)
		)
	`)
	if err != nil {
		return err
	}

	// _, err = tx.Exec(`
	// 	WITH valid_version_ids AS (
	// 		SELECT DISTINCT version_id FROM next_build_inputs
	// 	), valid_image_resource_version_ids AS (
	// 		SELECT i.id
	// 		FROM image_resource_versions i
	// 		WHERE build_id IN (
	// 			SELECT COALESCE(MAX(id), 0) AS build_id
	// 			FROM builds
	// 			WHERE status = 'succeeded'
	// 			GROUP BY job_id
	// 		)
	// 		OR build_id IN (
	// 			SELECT COALESCE(MAX(id), 0) AS build_id
	// 			FROM builds
	// 			GROUP BY job_id
	// 		)
	// 	), newly_inserted_version_caches AS (
	// 		INSERT INTO caches (version_id)
	// 		SELECT i.version_id
	// 		FROM valid_version_ids i
	// 		LEFT JOIN caches c
	// 		ON i.version_id = c.version_id
	// 		WHERE c.image_resource_version_id IS NULL
	// 		AND c.version_id IS NULL
	// 	), newly_inserted_image_caches AS (
	// 		INSERT INTO caches (image_resource_version_id)
	// 		SELECT i.id
	// 		FROM valid_image_resource_version_ids i
	// 		LEFT JOIN caches c
	// 		ON i.id = c.image_resource_version_id
	// 		WHERE c.image_resource_version_id IS NULL
	// 		AND c.version_id IS NULL
	// 	)
	// 	DELETE FROM caches
	// 	WHERE (
	// 		version_id IS NOT NULL
	// 		AND version_id NOT IN (
	// 			SELECT version_id FROM valid_version_ids
	// 		)
	// 	) OR (
	// 		image_resource_version_id IS NOT NULL
	// 		AND image_resource_version_id NOT IN (
	// 			SELECT id FROM valid_image_resource_version_ids
	// 		)
	// 	)
	// `)
	// if err != nil {
	// 	return err
	// }

	return nil
}
