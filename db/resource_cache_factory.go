package db

import (
	"database/sql"
	"encoding/json"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
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
	) (*UsedResourceCache, error)

	// changing resource cache to interface to allow updates on object is not feasible.
	// Since we need to pass it recursively in UsedResourceConfig.
	// Also, metadata will be available to us before we create resource cache so this
	// method can be removed at that point. See  https://github.com/concourse/concourse/issues/534
	UpdateResourceCacheMetadata(*UsedResourceCache, []atc.MetadataField) error
	ResourceCacheMetadata(*UsedResourceCache) (ResourceMetadataFields, error)
}

type resourceCacheFactory struct {
	conn Conn
}

func NewResourceCacheFactory(conn Conn) ResourceCacheFactory {
	return &resourceCacheFactory{
		conn: conn,
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
) (*UsedResourceCache, error) {
	resourceConfig, err := constructResourceConfig(resourceTypeName, source, resourceTypes)
	if err != nil {
		return nil, err
	}

	resourceCache := ResourceCache{
		ResourceConfig: resourceConfig,
		Version:        version,
		Params:         params,
	}

	var usedResourceCache *UsedResourceCache

	err = safeFindOrCreate(f.conn, func(tx Tx) error {
		var findOrCreateErr error
		usedResourceCache, findOrCreateErr = resourceCache.findOrCreate(logger, tx)
		if findOrCreateErr != nil {
			return findOrCreateErr
		}

		return resourceCache.use(logger, tx, usedResourceCache, resourceCacheUser)
	})

	if err != nil {
		return nil, err
	}

	return usedResourceCache, nil
}

func (f *resourceCacheFactory) UpdateResourceCacheMetadata(resourceCache *UsedResourceCache, metadata []atc.MetadataField) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = psql.Update("resource_caches").
		Set("metadata", metadataJSON).
		Where(sq.Eq{"id": resourceCache.ID}).
		RunWith(f.conn).
		Exec()
	return err
}

func (f *resourceCacheFactory) ResourceCacheMetadata(resourceCache *UsedResourceCache) (ResourceMetadataFields, error) {
	var metadataJSON sql.NullString
	err := psql.Select("metadata").
		From("resource_caches").
		Where(sq.Eq{"id": resourceCache.ID}).
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
