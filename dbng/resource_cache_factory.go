package dbng

import (
	"errors"

	"github.com/concourse/atc"
)

var ErrResourceTypeNotFound = errors.New("resource type not found")

//go:generate counterfeiter . ResourceCacheFactory

type ResourceCacheFactory interface {
	FindOrCreateResourceCacheForBuild(
		build *Build,
		resourceTypeName string,
		version atc.Version,
		source atc.Source,
		params atc.Params,
		pipeline *Pipeline,
		resourceTypes atc.ResourceTypes,
	) (*UsedResourceCache, error)

	FindOrCreateResourceCacheForResource(
		resource *Resource,
		resourceTypeName string,
		version atc.Version,
		source atc.Source,
		params atc.Params,
		pipeline *Pipeline,
		resourceTypes atc.ResourceTypes,
	) (*UsedResourceCache, error)

	FindOrCreateResourceCacheForResourceType(
		resourceTypeName string,
		version atc.Version,
		source atc.Source,
		params atc.Params,
		pipeline *Pipeline,
		resourceTypes atc.ResourceTypes,
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
	resourceTypeName string,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	pipeline *Pipeline,
	resourceTypes atc.ResourceTypes,
) (*UsedResourceCache, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	dbResourceTypes, err := getDBResourceTypes(tx, pipeline, resourceTypes)
	if err != nil {
		return nil, err
	}

	resourceConfig, err := constructResourceConfig(resourceTypeName, source, dbResourceTypes)
	if err != nil {
		return nil, err
	}

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

func (f *resourceCacheFactory) FindOrCreateResourceCacheForResource(
	resource *Resource,
	resourceTypeName string,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	pipeline *Pipeline,
	resourceTypes atc.ResourceTypes,
) (*UsedResourceCache, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	dbResourceTypes, err := getDBResourceTypes(tx, pipeline, resourceTypes)
	if err != nil {
		return nil, err
	}

	resourceConfig, err := constructResourceConfig(resourceTypeName, source, dbResourceTypes)
	if err != nil {
		return nil, err
	}

	resourceCache := ResourceCache{
		ResourceConfig: resourceConfig,
		Version:        version,
		Params:         params,
	}

	usedResourceCache, err := resourceCache.FindOrCreateForResource(tx, resource)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return usedResourceCache, nil
}

func (f *resourceCacheFactory) FindOrCreateResourceCacheForResourceType(
	resourceTypeName string,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	pipeline *Pipeline,
	resourceTypes atc.ResourceTypes,
) (*UsedResourceCache, error) {
	resourceType, found := resourceTypes.Lookup(resourceTypeName)
	if !found {
		return nil, ErrResourceTypeNotFound
	}

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	rt := ResourceType{
		ResourceType: resourceType,
		Pipeline:     pipeline,
	}

	usedResourceType, found, err := rt.Find(tx)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, ErrResourceTypeNotFound
	}

	dbResourceTypes, err := getDBResourceTypes(tx, pipeline, resourceTypes)
	if err != nil {
		return nil, err
	}

	resourceConfig, err := constructResourceConfig(resourceType.Name, source, dbResourceTypes)
	if err != nil {
		return nil, err
	}

	resourceCache := ResourceCache{
		ResourceConfig: resourceConfig,
		Version:        version,
		Params:         params,
	}

	usedResourceCache, err := resourceCache.FindOrCreateForResourceType(tx, usedResourceType)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return usedResourceCache, nil
}

func getDBResourceTypes(
	tx Tx,
	pipeline *Pipeline,
	resourceTypes atc.ResourceTypes,
) ([]ResourceType, error) {
	dbResourceTypes := []ResourceType{}
	for _, resourceType := range resourceTypes {
		dbResourceType := ResourceType{
			ResourceType: resourceType,
			Pipeline:     pipeline,
		}

		urt, found, err := dbResourceType.Find(tx)
		if err != nil {
			return nil, err
		}

		if !found {
			return nil, ErrResourceTypeNotFound
		}

		dbResourceType.Version = urt.Version
		dbResourceTypes = append(dbResourceTypes, dbResourceType)
	}
	return dbResourceTypes, nil
}
