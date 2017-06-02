package migrations

import (
	"database/sql"
	"encoding/json"

	"github.com/concourse/atc/db/migration"
)

type turbineMetadata struct {
	Guid     string `json:"guid"`
	Endpoint string `json:"endpoint"`
}

func AddEngineAndEngineMetadataToBuilds(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE builds ADD COLUMN engine varchar(16)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE builds ADD COLUMN engine_metadata text`)
	if err != nil {
		return err
	}

	cursor := 0

	for {
		var id int
		var guid, endpoint string

		err := tx.QueryRow(`
      SELECT id, guid, endpoint
      FROM builds
      WHERE id > $1
      AND guid != ''
      ORDER BY id ASC
      LIMIT 1
    `, cursor).Scan(&id, &guid, &endpoint)
		if err != nil {
			if err == sql.ErrNoRows {
				break
			}

			return err
		}

		cursor = id

		engineMetadata := turbineMetadata{
			Guid:     guid,
			Endpoint: endpoint,
		}

		payload, err := json.Marshal(engineMetadata)
		if err != nil {
			continue
		}

		_, err = tx.Exec(`
      UPDATE builds
      SET engine = $1, engine_metadata = $2
      WHERE id = $3
    `, "turbine", payload, id)
		if err != nil {
			continue
		}
	}

	_, err = tx.Exec(`ALTER TABLE builds DROP COLUMN guid`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE builds DROP COLUMN endpoint`)
	if err != nil {
		return err
	}

	return nil
}
