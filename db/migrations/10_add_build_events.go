package migrations

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"

	"github.com/BurntSushi/migration"
)

func AddBuildEvents(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TABLE build_events (
      id serial PRIMARY KEY,
      build_id integer REFERENCES builds (id),
			type varchar(32) NOT NULL,
      payload text NOT NULL
    )
  `)
	if err != nil {
		return err
	}

	cursor := 0

	for {
		var id int
		var buildLog sql.NullString

		err := tx.QueryRow(`
			SELECT id, log
			FROM builds
			WHERE id > $1
			LIMIT 1
		`, cursor).Scan(&id, &buildLog)
		if err != nil {
			if err == sql.ErrNoRows {
				break
			}

			return err
		}

		cursor = id

		if buildLog.Valid {
			decoder := json.NewDecoder(bytes.NewBufferString(buildLog.String))

			var version version
			err := decoder.Decode(&version)
			if err != nil {
				_, err = tx.Exec(`
					INSERT INTO build_events (build_id, type, payload)
					VALUES ($1, $2, $3)
				`, id, "version", "0.0")
				if err != nil {
					return err
				}

				_, err = tx.Exec(`
						INSERT INTO build_events (build_id, type, payload)
						VALUES ($1, $2, $3)
					`, id, "log", buildLog.String)
				if err != nil {
					return err
				}

				continue
			}

			_, err = tx.Exec(`
				INSERT INTO build_events (build_id, type, payload)
				VALUES ($1, $2, $3)
			`, id, "version", version.Version)
			if err != nil {
				return err
			}

			for {
				var event eventEnvelope
				err := decoder.Decode(&event)
				if err != nil {
					if err == io.EOF {
						break
					}

					return err
				}

				_, err = tx.Exec(`
					INSERT INTO build_events (build_id, type, payload)
					VALUES ($1, $2, $3)
				`, id, event.Type, []byte(*event.EventPayload))
				if err != nil {
					return err
				}
			}
		}
	}

	_, err = tx.Exec(`
		ALTER TABLE builds
		DROP COLUMN log
	`)
	if err != nil {
		return err
	}

	return nil
}

type eventEnvelope struct {
	Type         string           `json:"type"`
	EventPayload *json.RawMessage `json:"event"`
}

type version struct {
	Version string `json:"version"`
}
