package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
)

func Volume(volume dbng.CreatedVolume) (atc.Volume, error) {
	resourceType, err := volume.ResourceType()
	if err != nil {
		return atc.Volume{}, err
	}

	baseResourceType, err := volume.BaseResourceType()
	if err != nil {
		return atc.Volume{}, err
	}

	return atc.Volume{
		ID:               volume.Handle(),
		Type:             string(volume.Type()),
		WorkerName:       volume.Worker().Name(),
		SizeInBytes:      volume.SizeInBytes(),
		ContainerHandle:  volume.ContainerHandle(),
		Path:             volume.Path(),
		ParentHandle:     volume.ParentHandle(),
		ResourceType:     toVolumeResourceType(resourceType),
		BaseResourceType: toVolumeBaseResourceType(baseResourceType),
	}, nil
}

func toVolumeResourceType(dbResourceType *dbng.VolumeResourceType) *atc.VolumeResourceType {
	if dbResourceType == nil {
		return nil
	}

	if dbResourceType.BaseResourceType != nil {
		return &atc.VolumeResourceType{
			BaseResourceType: toVolumeBaseResourceType(dbResourceType.BaseResourceType),
			Version:          dbResourceType.Version,
		}
	}

	if dbResourceType.ResourceType != nil {
		resourceType := toVolumeResourceType(dbResourceType.ResourceType)
		return &atc.VolumeResourceType{
			ResourceType: resourceType,
			Version:      dbResourceType.Version,
		}
	}

	return nil
}

func toVolumeBaseResourceType(dbResourceType *dbng.WorkerBaseResourceType) *atc.VolumeBaseResourceType {
	if dbResourceType == nil {
		return nil
	}

	return &atc.VolumeBaseResourceType{
		Name:    dbResourceType.Name,
		Version: dbResourceType.Version,
	}
}
