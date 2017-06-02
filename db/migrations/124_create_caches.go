package migrations

import "github.com/concourse/atc/db/migration"

func CreateCaches(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE pipelines
		ALTER COLUMN name SET NOT NULL
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TYPE volume_state AS ENUM (
			'creating',
			'created',
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
		CREATE TABLE base_resource_types (
			id serial PRIMARY KEY,
			name text NOT NULL,
			UNIQUE (name)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE worker_base_resource_types (
			worker_name text REFERENCES workers (name) ON DELETE CASCADE,
			base_resource_type_id int REFERENCES base_resource_types (id) ON DELETE RESTRICT,
			image text NOT NULL,
			version text NOT NULL,
			UNIQUE (worker_name, base_resource_type_id)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE resource_configs (
			id serial PRIMARY KEY,
			base_resource_type_id int REFERENCES base_resource_types (id) ON DELETE CASCADE,
			source_hash text NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE resource_caches (
			id serial PRIMARY KEY,
			resource_config_id int REFERENCES resource_configs (id) ON DELETE CASCADE,
			version TEXT NOT NULL,
			params_hash text NOT NULL,
			UNIQUE (resource_config_id, version, params_hash)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resource_configs
		ADD COLUMN resource_cache_id int REFERENCES resource_caches (id) ON DELETE CASCADE,
		ADD UNIQUE (resource_cache_id, base_resource_type_id, source_hash)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE resource_config_uses (
			resource_config_id int REFERENCES resource_configs (id) ON DELETE RESTRICT,
			build_id int REFERENCES builds (id) ON DELETE CASCADE,
			resource_id int REFERENCES resources (id) ON DELETE CASCADE,
			resource_type_id int REFERENCES resource_types (id) ON DELETE CASCADE
			-- don't bother with unique constraint; easier to just blindly insert,
			-- and allow entries to just be GCed
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE resource_cache_uses (
			resource_cache_id int REFERENCES resource_caches (id) ON DELETE RESTRICT,
			build_id int REFERENCES builds (id) ON DELETE CASCADE,
			resource_id int REFERENCES resources (id) ON DELETE CASCADE,
			resource_type_id int REFERENCES resource_types (id) ON DELETE CASCADE
			-- don't bother with unique constraint; easier to just blindly insert,
			-- and allow entries to just be GCed
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE containers
		SET build_id = NULL
		WHERE build_id NOT IN (SELECT id FROM builds)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ALTER COLUMN handle DROP NOT NULL,
		ADD COLUMN state container_state NOT NULL DEFAULT 'created',
		ALTER COLUMN build_id SET DEFAULT NULL,
		ADD FOREIGN KEY (build_id) REFERENCES builds (id) ON DELETE SET NULL,
		ADD COLUMN resource_config_id int REFERENCES resource_configs (id) ON DELETE SET NULL,
		ADD COLUMN resource_cache_id int REFERENCES resource_caches (id) ON DELETE SET NULL,
		ADD COLUMN hijacked bool NOT NULL DEFAULT false,
		ADD CONSTRAINT handle_when_created CHECK (
			(state = 'creating' AND handle IS NULL) OR (state != 'creating')
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE containers
		ALTER COLUMN state SET DEFAULT 'creating'
	`)
	if err != nil {
		return err
	}

	// parent_state foreign key prevents any state changes on parent
	_, err = tx.Exec(`
		ALTER TABLE volumes
		ADD COLUMN resource_cache_id int REFERENCES resource_caches (id) ON DELETE SET NULL,
		ADD COLUMN base_resource_type_id int REFERENCES base_resource_types (id) ON DELETE SET NULL,
		ADD COLUMN state volume_state NOT NULL DEFAULT 'created',
		ADD COLUMN initialized bool NOT NULL DEFAULT false,
		ADD COLUMN parent_id int,
		ADD COLUMN parent_state volume_state,
		ADD UNIQUE (id, state),
		ADD FOREIGN KEY (parent_id, parent_state) REFERENCES volumes (id, state) ON DELETE RESTRICT,
		ADD CONSTRAINT cannot_invalidate_during_initialization CHECK (
			(
				state IN ('created', 'destroying') AND (
					(
						resource_cache_id IS NULL
					) AND (
						base_resource_type_id IS NULL
					) AND (
						container_id IS NULL
					)
				)
			) OR (
				(
					resource_cache_id IS NOT NULL
				) OR (
					base_resource_type_id IS NOT NULL
				) OR (
					container_id IS NOT NULL
				)
			)
		)
	`)
	if err != nil {
		return err
	}

	// https://www.pivotaltracker.com/story/show/144828721
	// All volumes that currently exist in the database have
	// already been initialized, and we rely on them being
	// initialized to GC them in the new schema.
	_, err = tx.Exec(`
		UPDATE volumes
		SET initialized = true
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE volumes
		ALTER COLUMN state SET DEFAULT 'creating'
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
