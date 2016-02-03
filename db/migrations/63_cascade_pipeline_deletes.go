package migrations

import "github.com/BurntSushi/migration"

func CascadePipelineDeletes(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE build_events DROP CONSTRAINT build_events_build_id_fkey;
		ALTER TABLE build_events ADD CONSTRAINT build_events_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds (id) ON DELETE CASCADE;

		ALTER TABLE build_outputs DROP CONSTRAINT build_outputs_build_id_fkey;
		ALTER TABLE build_outputs ADD CONSTRAINT build_outputs_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE;

		ALTER TABLE build_outputs DROP CONSTRAINT build_outputs_versioned_resource_id_fkey;
		ALTER TABLE build_outputs ADD CONSTRAINT build_outputs_versioned_resource_id_fkey FOREIGN KEY (versioned_resource_id) REFERENCES versioned_resources(id) ON DELETE CASCADE;

		ALTER TABLE build_inputs DROP CONSTRAINT build_inputs_build_id_fkey;
		ALTER TABLE build_inputs ADD CONSTRAINT build_inputs_build_id_fkey FOREIGN KEY (build_id) REFERENCES builds(id) ON DELETE CASCADE;

		ALTER TABLE build_inputs DROP CONSTRAINT build_inputs_versioned_resource_id_fkey;
		ALTER TABLE build_inputs ADD CONSTRAINT build_inputs_versioned_resource_id_fkey FOREIGN KEY (versioned_resource_id) REFERENCES versioned_resources(id) ON DELETE CASCADE;

		ALTER TABLE jobs_serial_groups DROP CONSTRAINT fkey_job_id;
		ALTER TABLE jobs_serial_groups ADD CONSTRAINT fkey_job_id FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;

		ALTER TABLE builds DROP CONSTRAINT fkey_job_id;
		ALTER TABLE builds ADD CONSTRAINT fkey_job_id FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;

		ALTER TABLE versioned_resources DROP CONSTRAINT fkey_resource_id;
		ALTER TABLE versioned_resources ADD CONSTRAINT fkey_resource_id FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE CASCADE;

		ALTER TABLE resources DROP CONSTRAINT resources_pipeline_id_fkey;
		ALTER TABLE resources ADD CONSTRAINT resources_pipeline_id_fkey FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE CASCADE;

		ALTER TABLE jobs DROP CONSTRAINT jobs_pipeline_id_fkey;
		ALTER TABLE jobs ADD CONSTRAINT jobs_pipeline_id_fkey FOREIGN KEY (pipeline_id) REFERENCES pipelines (id) ON DELETE CASCADE;
  `)

	return err
}
