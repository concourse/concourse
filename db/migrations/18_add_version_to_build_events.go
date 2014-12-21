package migrations

import (
	"encoding/json"

	"github.com/BurntSushi/migration"
)

type versionRange struct {
	version string
	start   int
}

func AddVersionToBuildEvents(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE build_events ADD COLUMN version text`)
	if err != nil {
		return err
	}

	versionRanges := map[int][]versionRange{}

	// collect version events to determine ranges
	versions, err := tx.Query(`
    SELECT build_id, payload, event_id
    FROM build_events
    WHERE type = 'version'
    ORDER BY event_id ASC
  `)
	if err != nil {
		return err
	}

	defer versions.Close()

	for versions.Next() {
		var buildID, eventID int
		var payload string
		err := versions.Scan(&buildID, &payload, &eventID)
		if err != nil {
			return err
		}

		// version event payload is e.g. '"1.0"'
		var version string
		err = json.Unmarshal([]byte(payload), &version)
		if err != nil {
			return err
		}

		versionRanges[buildID] = append(versionRanges[buildID], versionRange{
			version: version,
			start:   eventID,
		})
	}

	// close versions so that we can perform other operations in the transaction
	err = versions.Close()
	if err != nil {
		return err
	}

	// make event_id updates deferrable so we can decrement ranges of them
	_, err = tx.Exec(`
		ALTER TABLE build_events
		ADD CONSTRAINT deferrable_build_id_event_id
		UNIQUE USING INDEX build_events_build_id_event_id
		DEFERRABLE
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`SET CONSTRAINTS deferrable_build_id_event_id DEFERRED`)
	if err != nil {
		return err
	}

	// annotate each event with their version
	//
	// note that the order here matters as we don't bother to collect the end id.
	// this is guaranteed by the ORDER BY event_id ASC when collecting versions.
	for buildID, vranges := range versionRanges {
		for _, vrange := range vranges {
			// remove version event at this id
			_, err := tx.Exec(`
				DELETE FROM build_events
				WHERE build_id = $1
				AND event_id = $2
			`, buildID, vrange.start)
			if err != nil {
				return err
			}

			_, err = tx.Exec(`
	      UPDATE build_events
	      SET version = $2, event_id = event_id - 1
	      WHERE build_id = $1
	      AND event_id > $3
	    `, buildID, vrange.version, vrange.start)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
