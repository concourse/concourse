package dbng

import "github.com/concourse/atc"

//go:generate counterfeiter . ResourceConfigFactory

type ResourceConfigFactory interface {
	FindOrCreateResourceConfigForBuild(
		build *Build,
		resourceType string,
		source atc.Source,
		pipeline *Pipeline,
		resourceTypes atc.ResourceTypes,
	) (*UsedResourceConfig, error)

	FindOrCreateResourceConfigForResource(
		resource *Resource,
		resourceType string,
		source atc.Source,
		pipeline *Pipeline,
		resourceTypes atc.ResourceTypes,
	) (*UsedResourceConfig, error)

	FindOrCreateResourceConfigForResourceType(
		resourceTypeName string,
		source atc.Source,
		pipeline *Pipeline,
		resourceTypes atc.ResourceTypes,
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
	pipeline *Pipeline,
	resourceTypes atc.ResourceTypes,
) (*UsedResourceConfig, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	dbResourceTypes, err := getDBResourceTypes(tx, pipeline, resourceTypes)
	if err != nil {
		return nil, err
	}

	resourceConfig, err := constructResourceConfig(resourceType, source, dbResourceTypes)
	if err != nil {
		return nil, err
	}

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
	pipeline *Pipeline,
	resourceTypes atc.ResourceTypes,
) (*UsedResourceConfig, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	dbResourceTypes, err := getDBResourceTypes(tx, pipeline, resourceTypes)
	if err != nil {
		return nil, err
	}

	resourceConfig, err := constructResourceConfig(resourceType, source, dbResourceTypes)
	if err != nil {
		return nil, err
	}

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
	resourceTypeName string,
	source atc.Source,
	pipeline *Pipeline,
	resourceTypes atc.ResourceTypes,
) (*UsedResourceConfig, error) {
	resourceType, found := resourceTypes.Lookup(resourceTypeName)
	if !found {
		return nil, ErrResourceTypeNotFound
	}

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
