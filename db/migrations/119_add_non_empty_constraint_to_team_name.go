package migrations

import "github.com/concourse/atc/db/migration"

func AddNonEmptyConstraintToTeamName(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
ALTER TABLE teams
ADD CONSTRAINT constraint_teams_name_not_empty CHECK(length(name)>0)
    `)

	if err != nil {
		return err
	}

	return err
}
