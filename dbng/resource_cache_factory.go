package dbng

import "github.com/concourse/atc"

//go:generate counterfeiter . ResourceCacheFactory

type ResourceCacheFactory interface {
	FindOrCreateResourceCacheForBuild(
		build *Build,
		resourceType string,
		version atc.Version,
		source atc.Source,
		params atc.Params,
		resourceTypes []ResourceType,
	) (*UsedResourceCache, error)
}

type resourceCacheFactory struct {
	conn Conn
}

func NewResourceCacheFactory(conn Conn) ResourceCacheFactory {
	return &resourceCacheFactory{
		conn: conn,
	}
}

func (f *resourceCacheFactory) FindOrCreateResourceCacheForBuild(
	build *Build,
	resourceType string,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	resourceTypes []ResourceType,
) (*UsedResourceCache, error) {
	resourceConfig, err := constructResourceConfig(resourceType, source, resourceTypes)
	if err != nil {
		return nil, err
	}

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	resourceCache := ResourceCache{
		ResourceConfig: resourceConfig,
		Version:        version,
		Params:         params,
	}

	usedResourceCache, err := resourceCache.FindOrCreateForBuild(tx, build)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return usedResourceCache, nil
}
