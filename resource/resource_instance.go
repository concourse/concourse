package resource

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . ResourceInstance

type ResourceInstance interface {
	Source() atc.Source
	Params() atc.Params
	Version() atc.Version
	ResourceType() ResourceType
	ResourceUser() db.ResourceUser
	ContainerOwner() db.ContainerOwner

	LockName(string) (string, error)

	FindOn(lager.Logger, worker.Client) (worker.Volume, bool, error)
}

type resourceInstance struct {
	resourceTypeName       ResourceType
	version                atc.Version
	source                 atc.Source
	params                 atc.Params
	resourceUser           db.ResourceUser
	containerOwner         db.ContainerOwner
	resourceTypes          creds.VersionedResourceTypes
	dbResourceCacheFactory db.ResourceCacheFactory
}

func NewResourceInstance(
	resourceTypeName ResourceType,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	resourceUser db.ResourceUser,
	containerOwner db.ContainerOwner,
	resourceTypes creds.VersionedResourceTypes,
	dbResourceCacheFactory db.ResourceCacheFactory,
) ResourceInstance {
	return &resourceInstance{
		resourceTypeName:       resourceTypeName,
		version:                version,
		source:                 source,
		params:                 params,
		resourceUser:           resourceUser,
		containerOwner:         containerOwner,
		resourceTypes:          resourceTypes,
		dbResourceCacheFactory: dbResourceCacheFactory,
	}
}

func (instance resourceInstance) ResourceUser() db.ResourceUser {
	return instance.resourceUser
}

func (instance resourceInstance) ContainerOwner() db.ContainerOwner {
	return instance.containerOwner
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

func (instance resourceInstance) LockName(workerName string) (string, error) {
	id := &resourceInstanceLockID{
		Type:       instance.resourceTypeName,
		Version:    instance.version,
		Source:     instance.source,
		Params:     instance.params,
		WorkerName: workerName,
	}

	taskNameJSON, err := json.Marshal(id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(taskNameJSON)), nil
}

func (instance resourceInstance) FindOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, bool, error) {
	resourceCache, err := instance.dbResourceCacheFactory.FindOrCreateResourceCache(
		logger,
		instance.resourceUser,
		string(instance.resourceTypeName),
		instance.version,
		instance.source,
		instance.params,
		instance.resourceTypes,
	)
	if err != nil {
		logger.Error("failed-to-find-or-volume-resource-cache-for-build", err)
		return nil, false, err
	}

	return workerClient.FindVolumeForResourceCache(
		logger,
		resourceCache,
	)
}

type resourceInstanceLockID struct {
	Type       ResourceType `json:"type,omitempty"`
	Version    atc.Version  `json:"version,omitempty"`
	Source     atc.Source   `json:"source,omitempty"`
	Params     atc.Params   `json:"params,omitempty"`
	WorkerName string       `json:"worker_name,omitempty"`
}
