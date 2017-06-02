package migrations

import "github.com/concourse/atc/db/migration"

func AddCaseInsenstiveUniqueIndexToTeamsName(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
ALTER TABLE teams
DROP CONSTRAINT constraint_teams_name_unique
    `)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
	CREATE UNIQUE INDEX index_teams_name_unique_case_insensitive ON
	teams ( LOWER (name) )
	`)

	return err
}
