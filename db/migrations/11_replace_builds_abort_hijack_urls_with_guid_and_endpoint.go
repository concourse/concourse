package migrations

import (
	"database/sql"
	"net/url"
	"strings"

	"github.com/concourse/atc/db/migration"
)

func ReplaceBuildsAbortHijackURLsWithGuidAndEndpoint(tx migration.LimitedTx) error {
	_, err := tx.Exec(`ALTER TABLE builds ADD COLUMN guid varchar(36)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE builds ADD COLUMN endpoint varchar(128)`)
	if err != nil {
		return err
	}

	cursor := 0

	for {
		var id int
		var abortURLStr sql.NullString

		err := tx.QueryRow(`
			SELECT id, abort_url
			FROM builds
			WHERE id > $1
			LIMIT 1
		`, cursor).Scan(&id, &abortURLStr)
		if err != nil {
			if err == sql.ErrNoRows {
				break
			}

			return err
		}

		cursor = id

		if !abortURLStr.Valid {
			continue
		}

		// determine guid + endpoint from abort url
		//
		// format should be http://foo.com:5050/builds/some-guid/abort
		//
		// best-effort; skip if not possible, not a big deal

		abortURL, err := url.Parse(abortURLStr.String)
		if err != nil {
			continue
		}

		pathSegments := strings.Split(abortURL.Path, "/")
		if len(pathSegments) != 4 {
			continue
		}

		guid := pathSegments[2]
		endpoint := abortURL.Scheme + "://" + abortURL.Host

		_, err = tx.Exec(`
			UPDATE builds
			SET guid = $1, endpoint = $2
			WHERE id = $3
		`, guid, endpoint, id)
		if err != nil {
			continue
		}
	}

	_, err = tx.Exec(`ALTER TABLE builds DROP COLUMN abort_url`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`ALTER TABLE builds DROP COLUMN hijack_url`)
	if err != nil {
		return err
	}

	return nil
}
