package migrations

import "github.com/concourse/atc/db/migration"

func AddIndexesToABunchOfStuff(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE INDEX build_inputs_build_id_versioned_resource_id ON build_inputs (build_id, versioned_resource_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX build_outputs_build_id_versioned_resource_id ON build_outputs (build_id, versioned_resource_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX builds_job_id ON builds (job_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX jobs_pipeline_id ON jobs (pipeline_id);
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE INDEX resources_pipeline_id ON resources (pipeline_id);
	`)

	return nil
}
