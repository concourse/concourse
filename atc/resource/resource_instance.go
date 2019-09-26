package resource

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . ResourceInstance

type ResourceInstance interface {
	// XXX: do we need these?
	Source() atc.Source
	Params() atc.Params
	Version() atc.Version
	ResourceType() ResourceType

	ResourceCache() db.UsedResourceCache
	ContainerOwner() db.ContainerOwner

	//LockName(string) (string, error)

	//FindOn(lager.Logger, worker.Worker) (worker.Volume, bool, error)
}

type resourceInstance struct {
	resourceTypeName ResourceType
	version          atc.Version
	source           atc.Source
	params           atc.Params
	resourceTypes    atc.VersionedResourceTypes

	resourceCache  db.UsedResourceCache
	containerOwner db.ContainerOwner
}

func NewResourceInstance(
	resourceTypeName ResourceType,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	resourceTypes atc.VersionedResourceTypes,

	resourceCache db.UsedResourceCache,
	containerOwner db.ContainerOwner,
) ResourceInstance {
	return &resourceInstance{
		resourceTypeName: resourceTypeName,
		version:          version,
		source:           source,
		params:           params,
		resourceTypes:    resourceTypes,

		resourceCache:  resourceCache,
		containerOwner: containerOwner,
	}
}

func (instance resourceInstance) ContainerOwner() db.ContainerOwner {
	return instance.containerOwner
}

func (instance resourceInstance) ResourceCache() db.UsedResourceCache {
	return instance.resourceCache
}

func (instance resourceInstance) Source() atc.Source {
	return instance.source
}

func (instance resourceInstance) Params() atc.Params {
	return instance.params
}

func (instance resourceInstance) Version() atc.Version {
	return instance.version
}

func (instance resourceInstance) ResourceType() ResourceType {
	return instance.resourceTypeName
}

// XXX: this is weird
//func (instance resourceInstance) LockName(workerName string) (string, error) {
//	id := &resourceInstanceLockID{
//		Type:       instance.resourceTypeName,
//		Version:    instance.version,
//		Source:     instance.source,
//		Params:     instance.params,
//		WorkerName: workerName,
//	}
//
//	taskNameJSON, err := json.Marshal(id)
//	if err != nil {
//		return "", err
//	}
//	return fmt.Sprintf("%x", sha256.Sum256(taskNameJSON)), nil
//}

//func (instance resourceInstance) FindOn(logger lager.Logger, w worker.Worker) (worker.Volume, bool, error) {
//	return w.FindVolumeForResourceCache(
//		logger,
//		instance.resourceCache,
//	)
//}
