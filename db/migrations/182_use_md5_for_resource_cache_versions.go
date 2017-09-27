package migrations

import "github.com/concourse/atc/db/migration"

func UseMd5ForResourceCacheVersions(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE resource_caches DROP CONSTRAINT "resource_caches_resource_config_id_version_params_hash_key";
`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE UNIQUE INDEX resource_caches_resource_config_id_version_params_hash_key
		ON resource_caches (resource_config_id, md5(version), params_hash);
	`)
	if err != nil {
		return err
	}

	return nil
}
