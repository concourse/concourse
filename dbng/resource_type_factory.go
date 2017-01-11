package dbng

import "github.com/concourse/atc"

//go:generate counterfeiter . ResourceTypeFactory

type ResourceTypeFactory interface {
	FindResourceType(pipelineID int, resourceType atc.ResourceType) (*UsedResourceType, bool, error)
	CreateResourceType(pipelineID int, resourceType atc.ResourceType, version atc.Version) (*UsedResourceType, error)
}

type resourceTypeFactory struct {
	conn Conn
}

func NewResourceTypeFactory(conn Conn) ResourceTypeFactory {
	return &resourceTypeFactory{
		conn: conn,
	}
}

func (factory *resourceTypeFactory) FindResourceType(pipelineID int, resourceType atc.ResourceType) (*UsedResourceType, bool, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	rt := ResourceType{
		ResourceType: resourceType,
		PipelineID:   pipelineID,
	}

	urt, found, err := rt.Find(tx)
	if err != nil {
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return urt, found, nil
}

func (factory *resourceTypeFactory) CreateResourceType(pipelineID int, resourceType atc.ResourceType, version atc.Version) (*UsedResourceType, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	rt := ResourceType{
		ResourceType: resourceType,
		PipelineID:   pipelineID,
	}

	urt, err := rt.Create(tx, version)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return urt, nil
}
