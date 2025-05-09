package db

import (
	"database/sql"
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
)

//counterfeiter:generate . ResourceCacheFactory
type ResourceCacheFactory interface {
	FindOrCreateResourceCache(
		resourceCacheUser ResourceCacheUser,
		resourceTypeName string,
		version atc.Version,
		source atc.Source,
		params atc.Params,
		customTypeResourceCache ResourceCache,
	) (ResourceCache, error)

	// changing resource cache to interface to allow updates on object is not feasible.
	// Since we need to pass it recursively in ResourceConfig.
	// Also, metadata will be available to us before we create resource cache so this
	// method can be removed at that point. See  https://github.com/concourse/concourse/issues/534
	UpdateResourceCacheMetadata(ResourceCache, []atc.MetadataField) error
	ResourceCacheMetadata(ResourceCache) (ResourceConfigMetadataFields, error)

	FindResourceCacheByID(id int) (ResourceCache, bool, error)
}

type resourceCacheFactory struct {
	conn        DbConn
	lockFactory lock.LockFactory
}

func NewResourceCacheFactory(conn DbConn, lockFactory lock.LockFactory) ResourceCacheFactory {
	return &resourceCacheFactory{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

func (f *resourceCacheFactory) FindOrCreateResourceCache(
	resourceCacheUser ResourceCacheUser,
	resourceTypeName string,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	customTypeResourceCache ResourceCache,
) (ResourceCache, error) {
	rc := &resourceConfig{
		lockFactory: f.lockFactory,
		conn:        f.conn,
	}

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer Rollback(tx)

	err = findOrCreateResourceConfig(tx, rc, resourceTypeName, source, customTypeResourceCache, false)
	if err != nil {
		return nil, err
	}

	marshaledVersion, _ := json.Marshal(version)
	cacheVersion := string(marshaledVersion)

	found := true
	var id int
	err = psql.Select("id").
		From("resource_caches").
		Where(sq.Eq{
			"resource_config_id": rc.id,
			"params_hash":        paramsHash(params),
		}).
		Where(sq.Expr("version_sha256 = encode(digest(?, 'sha256'), 'hex')", cacheVersion)).
		Suffix("FOR SHARE").
		RunWith(tx).
		QueryRow().
		Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			found = false
		} else {
			return nil, err
		}
	}

	if !found {
		err = psql.Insert("resource_caches").
			Columns(
				"resource_config_id",
				"version",
				"version_sha256",
				"params_hash",
			).
			Values(
				rc.id,
				cacheVersion,
				sq.Expr("encode(digest(?, 'sha256'), 'hex')", cacheVersion),
				paramsHash(params),
			).
			Suffix(`
				ON CONFLICT (resource_config_id, version_sha256, params_hash) DO UPDATE SET
				resource_config_id = EXCLUDED.resource_config_id,
				version = EXCLUDED.version,
				version_sha256 = EXCLUDED.version_sha256,
				params_hash = EXCLUDED.params_hash
				RETURNING id
			`).
			RunWith(tx).
			QueryRow().
			Scan(&id)
		if err != nil {
			return nil, err
		}
	}

	cols := resourceCacheUser.SQLMap()
	cols["resource_cache_id"] = id

	found = true
	var resourceCacheUseExists int
	err = psql.Select("1").
		From("resource_cache_uses").
		Where(sq.Eq(cols)).
		RunWith(tx).
		QueryRow().
		Scan(&resourceCacheUseExists)
	if err != nil {
		if err == sql.ErrNoRows {
			found = false
		} else {
			return nil, err
		}
	}

	if !found {
		_, err = psql.Insert("resource_cache_uses").
			SetMap(cols).
			RunWith(tx).
			Exec()
		if err != nil {
			return nil, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &resourceCache{
		id:             id,
		resourceConfig: rc,
		version:        version,

		lockFactory: f.lockFactory,
		conn:        f.conn,
	}, nil
}

func (f *resourceCacheFactory) UpdateResourceCacheMetadata(resourceCache ResourceCache, metadata []atc.MetadataField) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = psql.Update("resource_caches").
		Set("metadata", metadataJSON).
		Where(sq.Eq{"id": resourceCache.ID()}).
		RunWith(f.conn).
		Exec()
	return err
}

func (f *resourceCacheFactory) ResourceCacheMetadata(resourceCache ResourceCache) (ResourceConfigMetadataFields, error) {
	var metadataJSON sql.NullString
	err := psql.Select("metadata").
		From("resource_caches").
		Where(sq.Eq{"id": resourceCache.ID()}).
		RunWith(f.conn).
		QueryRow().
		Scan(&metadataJSON)
	if err != nil {
		return nil, err
	}

	var metadata []ResourceConfigMetadataField
	if metadataJSON.Valid {
		err = json.Unmarshal([]byte(metadataJSON.String), &metadata)
		if err != nil {
			return nil, err
		}
	}

	return metadata, nil
}

func (f *resourceCacheFactory) FindResourceCacheByID(id int) (ResourceCache, bool, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer Rollback(tx)

	return findResourceCacheByID(tx, id, f.lockFactory, f.conn)
}

func findResourceCacheByID(tx Tx, resourceCacheID int, lock lock.LockFactory, conn DbConn) (ResourceCache, bool, error) {
	var rcID int
	var versionBytes string

	err := psql.Select("resource_config_id", "version").
		From("resource_caches").
		Where(sq.Eq{"id": resourceCacheID}).
		RunWith(tx).
		QueryRow().
		Scan(&rcID, &versionBytes)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	var version atc.Version
	err = json.Unmarshal([]byte(versionBytes), &version)
	if err != nil {
		return nil, false, err
	}

	rc, found, err := findResourceConfigByID(tx, rcID, lock, conn)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	usedResourceCache := &resourceCache{
		id:             resourceCacheID,
		version:        version,
		resourceConfig: rc,
		lockFactory:    lock,
		conn:           conn,
	}

	return usedResourceCache, true, nil
}
