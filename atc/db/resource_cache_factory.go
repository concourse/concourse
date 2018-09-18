package db

import (
	"database/sql"
	"encoding/json"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db/lock"
)

//go:generate counterfeiter . ResourceCacheFactory

type ResourceCacheFactory interface {
	FindOrCreateResourceCache(
		logger lager.Logger,
		resourceCacheUser ResourceCacheUser,
		resourceTypeName string,
		version atc.Version,
		source atc.Source,
		params atc.Params,
		resourceTypes creds.VersionedResourceTypes,
	) (UsedResourceCache, error)

	// changing resource cache to interface to allow updates on object is not feasible.
	// Since we need to pass it recursively in ResourceConfig.
	// Also, metadata will be available to us before we create resource cache so this
	// method can be removed at that point. See  https://github.com/concourse/concourse/issues/534
	UpdateResourceCacheMetadata(UsedResourceCache, []atc.MetadataField) error
	ResourceCacheMetadata(UsedResourceCache) (ResourceMetadataFields, error)
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
	logger lager.Logger,
	resourceCacheUser ResourceCacheUser,
	resourceTypeName string,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	resourceTypes creds.VersionedResourceTypes,
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

	usedResourceCache, err := resourceCache.findOrCreate(logger, tx, f.lockFactory, f.conn)
	if err != nil {
		return nil, err
	}

	err = resourceCache.use(logger, tx, usedResourceCache, resourceCacheUser)
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

func (f *resourceCacheFactory) ResourceCacheMetadata(resourceCache UsedResourceCache) (ResourceMetadataFields, error) {
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

	var metadata []ResourceMetadataField
	if metadataJSON.Valid {
		err = json.Unmarshal([]byte(metadataJSON.String), &metadata)
		if err != nil {
			return nil, err
		}
	}

	return metadata, nil
}
