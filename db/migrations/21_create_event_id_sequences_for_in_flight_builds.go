package migrations

import (
	"database/sql"
	"fmt"

	"github.com/BurntSushi/migration"
)

func CreateEventIDSequencesForInFlightBuilds(tx migration.LimitedTx) error {
	cursor := 0

	for {
		var id, eventIDStart int
		var guid, endpoint string

		err := tx.QueryRow(`
      SELECT id, max(event_id)
      FROM builds
      LEFT JOIN build_events
      ON build_id = id
      WHERE id > $1
      AND status = 'started'
      GROUP BY id
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

		_, err = tx.Exec(fmt.Sprintf(`
      CREATE SEQUENCE %s START WITH %d
    `, buildEventSeq(id), eventIDStart))
		if err != nil {
			continue
		}
	}

	return nil
}

func buildEventSeq(buildID int) string {
	return fmt.Sprintf("build_event_id_seq_%d", buildID)
}
