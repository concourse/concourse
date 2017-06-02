package migrations

import (
	"database/sql"

	"encoding/json"

	"crypto/sha256"
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/migration"
)

func AddMetadataToResourceCache(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE resource_caches
		ADD COLUMN metadata text
	`)
	if err != nil {
		return err
	}

	resourceCacheRows, err := tx.Query(`SELECT
		rca.id, version, source_hash, rcfg.base_resource_type_id, rcfg.resource_cache_id
		FROM resource_caches rca
		LEFT JOIN resource_configs rcfg
		ON rca.resource_config_id=rcfg.id`,
	)
	if err != nil {
		return err
	}

	defer resourceCacheRows.Close()

	resourceCacheInfos := []resourceCacheInfo{}

	for resourceCacheRows.Next() {
		var info resourceCacheInfo

		err = resourceCacheRows.Scan(&info.id, &info.version, &info.sourceHash, &info.baseResourceTypeID, &info.resourceCacheID)
		if err != nil {
			return err
		}
		resourceCacheInfos = append(resourceCacheInfos, info)
	}

	resourceTypesRows, err := tx.Query(`SELECT
		name, version, config
		FROM resource_types`,
	)
	if err != nil {
		return err
	}

	defer resourceTypesRows.Close()

	resourceTypesMap := make(resourceTypesInfo)

	for resourceTypesRows.Next() {
		var name string
		var version sql.NullString
		var config string

		err = resourceTypesRows.Scan(&name, &version, &config)
		if err != nil {
			return err
		}
		var resourceConfig atc.ResourceConfig
		err = json.Unmarshal([]byte(config), &resourceConfig)
		if err != nil {
			return err
		}

		sourceJSON, err := json.Marshal(resourceConfig.Source)
		if err != nil {
			return err
		}
		sourceHash := fmt.Sprintf("%x", sha256.Sum256(sourceJSON))

		if !version.Valid {
			// version is not set on resource type, ignore it, it does not have cached resources yet
			continue
		}

		resourceTypesMap[resourceTypeInfoKey{
			sourceHash: sourceHash,
			version:    version.String,
		}] = name
	}

	for _, info := range resourceCacheInfos {
		var resourceTypeName string
		// Base Resource Type
		if info.baseResourceTypeID.Valid {
			err = tx.QueryRow(
				"SELECT name FROM base_resource_types WHERE id = $1",
				info.baseResourceTypeID.Int64,
			).Scan(&resourceTypeName)
			if err != nil {
				return err
			}
		} else {
			var parentInfo resourceCacheInfo

			err = tx.QueryRow(
				`SELECT
				rca.id, version, source_hash, rcfg.base_resource_type_id, rcfg.resource_cache_id
				FROM resource_caches rca
				LEFT JOIN resource_configs rcfg
				ON rca.resource_config_id=rcfg.id
				WHERE rca.id = $1`,
				info.resourceCacheID.Int64,
			).Scan(
				&parentInfo.id, &parentInfo.version, &parentInfo.sourceHash, &parentInfo.baseResourceTypeID, &parentInfo.resourceCacheID,
			)
			if err != nil {
				return err
			}

			var ok bool
			resourceTypeName, ok = resourceTypesMap[resourceTypeInfoKey{
				sourceHash: parentInfo.sourceHash,
				version:    parentInfo.version,
			}]
			if !ok {
				// the resource type was deleted, while resource cache still exists
				continue
			}
		}

		var metadata string
		err = tx.QueryRow(
			"SELECT metadata FROM versioned_resources vr LEFT JOIN resources r ON vr.resource_id = r.id WHERE source_hash = $1 AND version = $2 AND type = $3",
			info.sourceHash,
			info.version,
			resourceTypeName,
		).Scan(&metadata)
		if err != nil {
			if err == sql.ErrNoRows {
				// this is custom resource type, which has resource cache but does not have versioned_resource
				continue
			}
			return err
		}

		_, err = tx.Exec("UPDATE resource_caches SET metadata = $1 WHERE id = $2", metadata, info.id)
		if err != nil {
			return err
		}
	}

	return nil
}

type resourceTypesInfo map[resourceTypeInfoKey]string

type resourceTypeInfoKey struct {
	sourceHash string
	version    string
}

type resourceCacheInfo struct {
	id                 int
	version            string
	sourceHash         string
	baseResourceTypeID sql.NullInt64
	resourceCacheID    sql.NullInt64
}
