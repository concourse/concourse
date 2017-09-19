package migrations

import "github.com/concourse/atc/db/migration"

func AddViewsForBuildStates(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE MATERIALIZED VIEW latest_completed_builds_per_job AS
		WITH latest_build_ids_per_job AS (
			SELECT MAX(b.id) AS build_id
			FROM builds b
			INNER JOIN jobs j ON j.id = b.job_id
			WHERE b.status NOT IN ('pending', 'started')
			GROUP BY b.job_id
		)
		SELECT b.*
		FROM builds b
		INNER JOIN latest_build_ids_per_job l ON l.build_id = b.id
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE MATERIALIZED VIEW next_builds_per_job AS
		WITH latest_build_ids_per_job AS (
			SELECT MIN(b.id) AS build_id
			FROM builds b
			INNER JOIN jobs j ON j.id = b.job_id
			WHERE b.status IN ('pending', 'started')
			GROUP BY b.job_id
		)
		SELECT b.*
		FROM builds b
		INNER JOIN latest_build_ids_per_job l ON l.build_id = b.id
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE MATERIALIZED VIEW transition_builds_per_job AS
		WITH builds_before_transition AS (
			SELECT b.job_id, MAX(b.id)
			FROM builds b
			LEFT OUTER JOIN jobs j ON (b.job_id = j.id)
			LEFT OUTER JOIN latest_completed_builds_per_job s ON b.job_id = s.job_id
			WHERE b.status != s.status
			AND b.status NOT IN ('pending', 'started')
			GROUP BY b.job_id
		)
		SELECT DISTINCT ON (b.job_id) b.*
		FROM builds b
		LEFT OUTER JOIN builds_before_transition ON b.job_id = builds_before_transition.job_id
		WHERE builds_before_transition.max IS NULL
		AND b.status NOT IN ('pending', 'started')
		OR b.id > builds_before_transition.max
		ORDER BY b.job_id, b.id ASC
	`)
	if err != nil {
		return err
	}

	return nil
}
