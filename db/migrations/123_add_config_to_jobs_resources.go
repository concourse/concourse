package migrations

import (
	"encoding/json"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/migration"
)

func AddConfigToJobsResources(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE resources
		ADD COLUMN config json NOT NULL DEFAULT '{}',
		ADD COLUMN active bool NOT NULL DEFAULT false;
 	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resource_types
		ADD COLUMN config json NOT NULL DEFAULT '{}',
		ADD COLUMN active bool NOT NULL DEFAULT false;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs
		ADD COLUMN config json NOT NULL DEFAULT '{}',
		ADD COLUMN active bool NOT NULL DEFAULT false;
	`)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`
    SELECT id, config
  	FROM pipelines
  `)
	if err != nil {
		return err
	}

	defer rows.Close()

	pipelineConfigs := map[int]atc.Config{}

	for rows.Next() {
		var pipelineID int
		var pipelineConfigPayload []byte
		err := rows.Scan(&pipelineID, &pipelineConfigPayload)
		if err != nil {
			return err
		}

		var pipelineConfig atc.Config
		err = json.Unmarshal(pipelineConfigPayload, &pipelineConfig)
		if err != nil {
			return err
		}

		pipelineConfigs[pipelineID] = pipelineConfig
	}

	for pipelineID, pipelineConfig := range pipelineConfigs {
		for _, jobConfig := range pipelineConfig.Jobs {
			jobConfigPayload, err := json.Marshal(jobConfig)
			if err != nil {
				return err
			}

			_, err = tx.Exec(`
				UPDATE jobs
				SET config = $1, active = true
				WHERE name = $2 AND pipeline_id = $3
			`, jobConfigPayload, jobConfig.Name, pipelineID)
			if err != nil {
				return err
			}

			for _, resourceConfig := range pipelineConfig.Resources {
				resourceConfigPayload, err := json.Marshal(resourceConfig)
				if err != nil {
					return err
				}

				_, err = tx.Exec(`
					UPDATE resources
					SET config = $1, active = true
					WHERE name = $2 AND pipeline_id = $3
			      `, resourceConfigPayload, resourceConfig.Name, pipelineID)
				if err != nil {
					return err
				}
			}

			for _, resourceTypeConfig := range pipelineConfig.ResourceTypes {
				resourceTypeConfigPayload, err := json.Marshal(resourceTypeConfig)
				if err != nil {
					return err
				}

				_, err = tx.Exec(`
					UPDATE resource_types
					SET config = $1, active = true
					WHERE name = $2 AND pipeline_id = $3
				 `, resourceTypeConfigPayload, resourceTypeConfig.Name, pipelineID)
				if err != nil {
					return err
				}
			}
		}
	}

	_, err = tx.Exec(`
		ALTER TABLE resources ALTER COLUMN config DROP DEFAULT;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE resource_types ALTER COLUMN config DROP DEFAULT;
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE jobs ALTER COLUMN config DROP DEFAULT;
	`)
	if err != nil {
		return err
	}

	return nil
}
