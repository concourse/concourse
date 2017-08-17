package migrations

import "github.com/concourse/atc/db/migration"

func AddOnDeleteRestrictToResourceConfigsAndCachesAndResourceConfigCheckSessions(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE resource_configs
    DROP CONSTRAINT resource_configs_resource_cache_id_fkey;
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE resource_configs
    ADD CONSTRAINT resource_configs_resource_cache_id_fkey
    FOREIGN KEY (resource_cache_id)
    REFERENCES resource_caches (id) ON DELETE RESTRICT;
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
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

	_, err = tx.Exec(`
    ALTER TABLE resource_config_check_sessions
    DROP CONSTRAINT resource_config_check_sessions_resource_config_id_fkey;
  `)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE resource_config_check_sessions
    ADD CONSTRAINT resource_config_check_sessions_resource_config_id_fkey
    FOREIGN KEY (resource_config_id)
    REFERENCES resource_configs (id) ON DELETE RESTRICT;
  `)
	if err != nil {
		return err
	}

	return nil
}
