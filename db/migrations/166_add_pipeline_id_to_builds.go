package migrations

import "github.com/concourse/atc/db/migration"

func AddPipelineIdToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE builds
  	ADD COLUMN pipeline_id integer,
  	ADD FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE CASCADE;
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
  	UPDATE builds
    SET pipeline_id = (SELECT j.pipeline_id FROM jobs j WHERE j.id = builds.job_id);
		`)
	if err != nil {
		return err
	}

	return nil
}
