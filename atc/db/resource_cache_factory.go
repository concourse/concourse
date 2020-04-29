package db

import (
	"database/sql"
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . ResourceCacheFactory

type ResourceCacheFactory interface {
	FindOrCreateResourceCache(
		resourceCacheUser ResourceCacheUser,
		resourceTypeName string,
		version atc.Version,
		source atc.Source,
		params atc.Params,
		resourceTypes atc.VersionedResourceTypes,
	) (UsedResourceCache, error)

	// changing resource cache to interface to allow updates on object is not feasible.
	// Since we need to pass it recursively in ResourceConfig.
	// Also, metadata will be available to us before we create resource cache so this
	// method can be removed at that point. See  https://github.com/concourse/concourse/issues/534
	UpdateResourceCacheMetadata(UsedResourceCache, []atc.MetadataField) error
	ResourceCacheMetadata(UsedResourceCache) (ResourceConfigMetadataFields, error)

	FindResourceCacheByID(id int) (UsedResourceCache, bool, error)
}

type resourceCacheFactory struct {
	conn        Conn
	lockFactory lock.LockFactory
}

func NewResourceCacheFactory(conn Conn, lockFactory lock.LockFactory) ResourceCacheFactory {
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
	resourceTypes atc.VersionedResourceTypes,
) (UsedResourceCache, error) {
	resourceConfigDescriptor, err := constructResourceConfigDescriptor(resourceTypeName, source, resourceTypes)
	if err != nil {
		return nil, err
	}

	resourceCache := ResourceCacheDescriptor{
		ResourceConfigDescriptor: resourceConfigDescriptor,
		Version:                  version,
		Params:                   params,
	}

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	usedResourceCache, err := resourceCache.findOrCreate(tx, f.lockFactory, f.conn)
	if err != nil {
		return nil, err
	}

	err = resourceCache.use(tx, usedResourceCache, resourceCacheUser)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return usedResourceCache, nil
}

func (f *resourceCacheFactory) UpdateResourceCacheMetadata(resourceCache UsedResourceCache, metadata []atc.MetadataField) error {
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

func (f *resourceCacheFactory) ResourceCacheMetadata(resourceCache UsedResourceCache) (ResourceConfigMetadataFields, error) {
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

func (f *resourceCacheFactory) FindResourceCacheByID(id int) (UsedResourceCache, bool, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer Rollback(tx)

	return findResourceCacheByID(tx, id, f.lockFactory, f.conn)
}

func findResourceCacheByID(tx Tx, resourceCacheID int, lock lock.LockFactory, conn Conn) (UsedResourceCache, bool, error) {
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

	usedResourceCache := &usedResourceCache{
		id:             resourceCacheID,
		version:        version,
		resourceConfig: rc,
		lockFactory:    lock,
		conn:           conn,
	}

	return usedResourceCache, true, nil
}
