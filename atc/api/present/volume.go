package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func Volume(volume db.CreatedVolume) (atc.Volume, error) {
	resourceType, err := volume.ResourceType()
	if err != nil {
		return atc.Volume{}, err
	}

	baseResourceType, err := volume.BaseResourceType()
	if err != nil {
		return atc.Volume{}, err
	}

	pipelineName, jobName, stepName, err := volume.TaskIdentifier()
	if err != nil {
		return atc.Volume{}, err
	}

	return atc.Volume{
		ID:               volume.Handle(),
		Type:             string(volume.Type()),
		WorkerName:       volume.WorkerName(),
		ContainerHandle:  volume.ContainerHandle(),
		Path:             volume.Path(),
		ParentHandle:     volume.ParentHandle(),
		PipelineName:     pipelineName,
		JobName:          jobName,
		StepName:         stepName,
		ResourceType:     toVolumeResourceType(resourceType),
		BaseResourceType: toVolumeBaseResourceType(baseResourceType),
	}, nil
}

func toVolumeResourceType(dbResourceType *db.VolumeResourceType) *atc.VolumeResourceType {
	if dbResourceType == nil {
		return nil
	}

	if dbResourceType.WorkerBaseResourceType != nil {
		return &atc.VolumeResourceType{
			BaseResourceType: toVolumeBaseResourceType(dbResourceType.WorkerBaseResourceType),
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

func toVolumeBaseResourceType(dbResourceType *db.UsedWorkerBaseResourceType) *atc.VolumeBaseResourceType {
	if dbResourceType == nil {
		return nil
	}

	return &atc.VolumeBaseResourceType{
		Name:    dbResourceType.Name,
		Version: dbResourceType.Version,
	}
}
