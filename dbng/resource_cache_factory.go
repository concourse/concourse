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
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	resourceCache, err := f.constructResourceCache(resourceType, version, source, params, resourceTypes)
	if err != nil {
		return nil, err
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

func (f *resourceCacheFactory) constructResourceCache(
	resourceType string,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	resourceTypes []ResourceType,
) (ResourceCache, error) {
	resourceCache := ResourceCache{
		ResourceConfig: ResourceConfig{
			Source: source,
		},
		Version: version,
		Params:  params,
	}

	resourceTypesList := resourceTypesList(resourceType, resourceTypes, []ResourceType{})
	if len(resourceTypesList) == 0 {
		resourceCache.ResourceConfig.CreatedByBaseResourceType = &BaseResourceType{
			Name: resourceType,
		}
	} else {
		lastResourceType := resourceTypesList[len(resourceTypesList)-1]

		parentResourceCache := &ResourceCache{
			ResourceConfig: ResourceConfig{
				CreatedByBaseResourceType: &BaseResourceType{
					Name: lastResourceType.Type,
				},
				Source: lastResourceType.Source,
			},
			Version: lastResourceType.Version,
		}

		for i := len(resourceTypesList) - 2; i >= 0; i-- {
			parentResourceCache = &ResourceCache{
				ResourceConfig: ResourceConfig{
					CreatedByResourceCache: parentResourceCache,
					Source:                 resourceTypesList[i].Source,
				},
				Version: resourceTypesList[i].Version,
			}
		}

		resourceCache.ResourceConfig.CreatedByResourceCache = parentResourceCache
	}

	return resourceCache, nil
}

func resourceTypesList(resourceTypeName string, allResourceTypes []ResourceType, resultResourceTypes []ResourceType) []ResourceType {
	for _, resourceType := range allResourceTypes {
		if resourceType.Name == resourceTypeName {
			resultResourceTypes = append(resultResourceTypes, resourceType)
			return resourceTypesList(resourceType.Type, allResourceTypes, resultResourceTypes)
		}
	}

	return resultResourceTypes
}
