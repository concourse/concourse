package migrations

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"

	"github.com/concourse/atc/db/migration"
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
			ORDER BY id ASC
			LIMIT 1
		`, cursor).Scan(&id, &buildLog)
		if err != nil {
			if err == sql.ErrNoRows {
				break
			}

			return err
		}

		cursor = id

		if !buildLog.Valid {
			continue
		}

		logBuf := bytes.NewBufferString(buildLog.String)
		decoder := json.NewDecoder(logBuf)

		for {
			var entry logEntry

			err := decoder.Decode(&entry)
			if err != nil {
				if err != io.EOF {
					// non-JSON log; assume v0.0

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
				}

				break
			}

			if entry.Type != "" && entry.EventPayload != nil {
				_, err = tx.Exec(`
						INSERT INTO build_events (build_id, type, payload)
						VALUES ($1, $2, $3)
					`, id, entry.Type, []byte(*entry.EventPayload))
				if err != nil {
					return err
				}

				continue
			}

			if entry.Version != "" {
				versionEnc, err := json.Marshal(entry.Version)
				if err != nil {
					return err
				}

				_, err = tx.Exec(`
					INSERT INTO build_events (build_id, type, payload)
					VALUES ($1, $2, $3)
				`, id, "version", versionEnc)
				if err != nil {
					return err
				}

				continue
			}

			return fmt.Errorf("malformed event stream; got stuck at %s", logBuf.String())
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

type logEntry struct {
	// either an event...
	Type         string           `json:"type"`
	EventPayload *json.RawMessage `json:"event"`

	// ...or a version
	Version string `json:"version"`
}
