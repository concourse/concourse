package migrations

import (
	"database/sql"
	"fmt"

	"github.com/concourse/atc/db/migration"
)

func CreateEventIDSequencesForInFlightBuilds(tx migration.LimitedTx) error {
	cursor := 0

	for {
		var id, eventIDStart int

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
    `, cursor).Scan(&id, &eventIDStart)
		if err != nil {
			if err == sql.ErrNoRows {
				break
			}

			return err
		}

		cursor = id

		_, err = tx.Exec(fmt.Sprintf(`
      CREATE SEQUENCE %s MINVALUE 0 START WITH %d
    `, buildEventSeq(id), eventIDStart+1))
		if err != nil {
			return err
		}
	}

	return nil
}

func buildEventSeq(buildID int) string {
	return fmt.Sprintf("build_event_id_seq_%d", buildID)
}
