package dbng

import "github.com/concourse/atc"

//go:generate counterfeiter . ResourceConfigFactory

type ResourceConfigFactory interface {
	FindOrCreateResourceConfigForBuild(
		build *Build,
		resourceType string,
		source atc.Source,
		resourceTypes []ResourceType,
	) (*UsedResourceConfig, error)

	FindOrCreateResourceConfigForResource(
		resource *Resource,
		resourceType string,
		source atc.Source,
		resourceTypes []ResourceType,
	) (*UsedResourceConfig, error)

	FindOrCreateResourceConfigForResourceType(
		usedResourceType *UsedResourceType,
		resourceType string,
		source atc.Source,
		resourceTypes []ResourceType,
	) (*UsedResourceConfig, error)
}

type resourceConfigFactory struct {
	conn Conn
}

func NewResourceConfigFactory(conn Conn) ResourceConfigFactory {
	return &resourceConfigFactory{
		conn: conn,
	}
}

func (f *resourceConfigFactory) FindOrCreateResourceConfigForBuild(
	build *Build,
	resourceType string,
	source atc.Source,
	resourceTypes []ResourceType,
) (*UsedResourceConfig, error) {
	resourceConfig, err := constructResourceConfig(resourceType, source, resourceTypes)
	if err != nil {
		return nil, err
	}

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	usedResourceConfig, err := resourceConfig.FindOrCreateForBuild(tx, build)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return usedResourceConfig, nil
}

func (f *resourceConfigFactory) FindOrCreateResourceConfigForResource(
	resource *Resource,
	resourceType string,
	source atc.Source,
	resourceTypes []ResourceType,
) (*UsedResourceConfig, error) {
	resourceConfig, err := constructResourceConfig(resourceType, source, resourceTypes)
	if err != nil {
		return nil, err
	}

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	usedResourceConfig, err := resourceConfig.FindOrCreateForResource(tx, resource)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return usedResourceConfig, nil
}

func (f *resourceConfigFactory) FindOrCreateResourceConfigForResourceType(
	usedResourceType *UsedResourceType,
	resourceType string,
	source atc.Source,
	resourceTypes []ResourceType,
) (*UsedResourceConfig, error) {
	resourceConfig, err := constructResourceConfig(resourceType, source, resourceTypes)
	if err != nil {
		return nil, err
	}

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	usedResourceConfig, err := resourceConfig.FindOrCreateForResourceType(tx, usedResourceType)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return usedResourceConfig, nil
}

func constructResourceConfig(
	resourceType string,
	source atc.Source,
	resourceTypes []ResourceType,
) (ResourceConfig, error) {
	resourceConfig := ResourceConfig{
		Source: source,
	}

	resourceTypesList := resourceTypesList(resourceType, resourceTypes, []ResourceType{})
	if len(resourceTypesList) == 0 {
		resourceConfig.CreatedByBaseResourceType = &BaseResourceType{
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

		resourceConfig.CreatedByResourceCache = parentResourceCache
	}

	return resourceConfig, nil
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
