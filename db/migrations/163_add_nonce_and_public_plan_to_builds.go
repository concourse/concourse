package migrations

import (
	"encoding/json"
	"fmt"

	"github.com/concourse/atc/db/migration"
	internal "github.com/concourse/atc/db/migrations/internal/163"
)

func AddNonceAndPublicPlanToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE builds
		ADD COLUMN nonce text;
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE builds
		ADD COLUMN public_plan json DEFAULT '{}';
`)
	if err != nil {
		return err
	}

	offset := 0
	for {
		rows, err := tx.Query(fmt.Sprintf(`
		SELECT id, engine_metadata
		FROM builds
		WHERE engine='exec.v2'
		ORDER BY id ASC
		LIMIT 500
		OFFSET %d
	`, offset))
		if err != nil {
			return err
		}

		defer rows.Close()

		//create public plans
		plans := map[int]internal.Plan{}

		totalRows := 0
		for rows.Next() {
			totalRows++

			var buildID int
			var engineMetadataJSON []byte
			err := rows.Scan(&buildID, &engineMetadataJSON)
			if err != nil {
				return err
			}

			if engineMetadataJSON == nil {
				continue
			}

			var execEngineMetadata execV2Metadata
			err = json.Unmarshal(engineMetadataJSON, &execEngineMetadata)
			if err != nil {
				return err
			}

			plans[buildID] = execEngineMetadata.Plan
		}

		if totalRows == 0 {
			break
		} else {
			offset += totalRows
		}

		for buildID, plan := range plans {
			_, err := tx.Exec(`
				UPDATE builds
				SET
				  public_plan = $1
				WHERE
					id = $2
			`, plan.Public(), buildID)
			if err != nil {
				return err
			}
		}
	}

	_, err = tx.Exec(`
		UPDATE builds
		  SET
			  engine_metadata = NULL
			WHERE
			  engine = 'exec.v2' AND
				status IN ('succeeded','aborted','failed','errored')
	`)

	return err
}

type execV2Metadata struct {
	Plan internal.Plan
}
