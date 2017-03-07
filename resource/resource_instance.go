package resource

import (
	"crypto/sha512"
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/worker"
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
			Strategy: worker.ResourceCacheStrategy{
				ResourceHash:    GenerateResourceHash(instance.source, string(instance.resourceTypeName)),
				ResourceVersion: instance.version,
			},
			Properties: instance.volumeProperties(),
			Privileged: true,
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

func (instance resourceInstance) volumeProperties() worker.VolumeProperties {
	source, _ := json.Marshal(instance.source)

	version, _ := json.Marshal(instance.version)

	params, _ := json.Marshal(instance.params)

	return worker.VolumeProperties{
		"resource-type":    string(instance.resourceTypeName),
		"resource-version": string(version),
		"resource-source":  shastr(source),
		"resource-params":  shastr(params),
	}
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

func shastr(b []byte) string {
	return fmt.Sprintf("%x", sha512.Sum512(b))
}

func selectLowestAlphabeticalVolume(logger lager.Logger, volumes []worker.Volume) worker.Volume {
	var lowestVolume worker.Volume

	for _, v := range volumes {
		if lowestVolume == nil {
			lowestVolume = v
		} else if v.Handle() < lowestVolume.Handle() {
			lowestVolume = v
		}
	}

	for _, v := range volumes {
		if v != lowestVolume {
			v.Destroy()
		}
	}

	return lowestVolume
}
