package migrations

import "github.com/concourse/atc/db/migration"

func DropResourceConfigUsesAndAddContainerIDToResourceCacheUsesWhileAlsoRemovingResourceIDAndResourceTypeIDFromResourceCacheUses(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    DROP TABLE resource_config_uses
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE resource_cache_uses
    DROP COLUMN resource_id,
    DROP COLUMN resource_type_id,
		ADD COLUMN container_id integer REFERENCES containers (id) ON DELETE CASCADE
  `)
	if err != nil {
		return err
	}

	return nil
}
