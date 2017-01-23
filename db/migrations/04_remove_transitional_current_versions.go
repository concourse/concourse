package migrations

import "github.com/concourse/atc/dbng/migration"

func RemoveTransitionalCurrentVersions(tx migration.LimitedTx) error {
	_, err := tx.Exec("DROP TABLE transitional_current_versions")
	if err != nil {
		return err
	}

	return nil
}
