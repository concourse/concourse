package migrations

import "github.com/concourse/atc/db/migration"

func CleanUpContainerColumns(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE containers
		ADD COLUMN meta_type text NOT NULL DEFAULT '',
		ADD COLUMN meta_step_name text NOT NULL DEFAULT '',
		ADD COLUMN meta_attempt text NOT NULL DEFAULT '',
		ADD COLUMN meta_working_directory text NOT NULL DEFAULT '',
		ADD COLUMN meta_process_user text NOT NULL DEFAULT '',
		ADD COLUMN meta_pipeline_id integer NOT NULL DEFAULT 0,
		ADD COLUMN meta_job_id integer NOT NULL DEFAULT 0,
		ADD COLUMN meta_build_id integer NOT NULL DEFAULT 0,
		ADD COLUMN meta_pipeline_name text NOT NULL DEFAULT '',
		ADD COLUMN meta_job_name text NOT NULL DEFAULT '',
		ADD COLUMN meta_build_name text NOT NULL DEFAULT '',
		DROP COLUMN type,
		DROP COLUMN step_name,
		DROP COLUMN check_type,
		DROP COLUMN check_source,
		DROP COLUMN working_directory,
		DROP COLUMN env_variables,
		DROP COLUMN attempts,
		DROP COLUMN stage,
		DROP COLUMN image_resource_type,
		DROP COLUMN image_resource_source,
		DROP COLUMN process_user,
		DROP COLUMN resource_type_version
	`)
	if err != nil {
		return err
	}

	return nil
}
