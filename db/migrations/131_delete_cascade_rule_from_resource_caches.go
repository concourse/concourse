package migrations

import "github.com/BurntSushi/migration"

func DeleteCascadeRuleFromResourceCaches(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE resource_caches
    DROP CONSTRAINT resource_caches_resource_config_id_fkey;
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE resource_caches
    ADD CONSTRAINT resource_caches_resource_config_id_fkey
    FOREIGN KEY (resource_config_id)
    REFERENCES resource_configs (id) ON DELETE RESTRICT;
  `)
	if err != nil {
		return err
	}

	return nil
}
