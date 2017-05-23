package resource

import (
	"encoding/json"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"
)

//go:generate counterfeiter . ResourceInstance

type ResourceInstance interface {
	ResourceUser() dbng.ResourceUser

	FindInitializedOn(lager.Logger, worker.Client) (worker.Volume, bool, error)
	CreateOn(lager.Logger, worker.Client) (worker.Volume, error)

	ResourceCacheIdentifier() worker.ResourceCacheIdentifier
}

type resourceInstance struct {
	resourceTypeName       ResourceType
	version                atc.Version
	source                 atc.Source
	params                 atc.Params
	resourceUser           dbng.ResourceUser
	resourceTypes          atc.VersionedResourceTypes
	dbResourceCacheFactory dbng.ResourceCacheFactory
}

func NewResourceInstance(
	resourceTypeName ResourceType,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	resourceUser dbng.ResourceUser,
	resourceTypes atc.VersionedResourceTypes,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
) ResourceInstance {
	return &resourceInstance{
		resourceTypeName:       resourceTypeName,
		version:                version,
		source:                 source,
		params:                 params,
		resourceUser:           resourceUser,
		resourceTypes:          resourceTypes,
		dbResourceCacheFactory: dbResourceCacheFactory,
	}
}

func (instance resourceInstance) ResourceUser() dbng.ResourceUser {
	return instance.resourceUser
}

func (instance resourceInstance) CreateOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, error) {
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
		return nil, err
	}

	return workerClient.CreateVolumeForResourceCache(
		logger,
		worker.VolumeSpec{
			Strategy: baggageclaim.EmptyStrategy{},
		},
		resourceCache,
	)
}

func (instance resourceInstance) FindInitializedOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, bool, error) {
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
		logger.Error("failed-to-find-or-initialized-volume-resource-cache-for-build", err)
		return nil, false, err
	}

	return workerClient.FindInitializedVolumeForResourceCache(
		logger,
		resourceCache,
	)
}

func (instance resourceInstance) ResourceCacheIdentifier() worker.ResourceCacheIdentifier {
	return worker.ResourceCacheIdentifier{
		ResourceVersion: instance.version,
		ResourceHash:    GenerateResourceHash(instance.source, string(instance.resourceTypeName)),
	}
}

func GenerateResourceHash(source atc.Source, resourceType string) string {
	sourceJSON, _ := json.Marshal(source)
	return resourceType + string(sourceJSON)
}
