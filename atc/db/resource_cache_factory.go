package db

import (
	"database/sql"
	"encoding/json"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db/lock"
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
