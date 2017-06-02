package migrations

import "github.com/concourse/atc/db/migration"

func RemoveDuplicateIndices(tx migration.LimitedTx) error {
	_, err := tx.Exec(`DROP INDEX builds_job_id_idx, jobs_pipeline_id_idx, resources_pipeline_id_idx, pipelines_team_id_idx`)
	if err != nil {
		return err
	}

	return nil
}
