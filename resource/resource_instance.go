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
	FindOn(lager.Logger, worker.Client) (worker.Volume, bool, error)
	FindOrCreateOn(lager.Logger, worker.Client) (worker.Volume, error)

	ResourceCacheIdentifier() worker.ResourceCacheIdentifier
}

type buildResourceInstance struct {
	resourceInstance
	buildID                int
	pipelineID             int
	resourceTypes          atc.ResourceTypes
	dbResourceCacheFactory dbng.ResourceCacheFactory
}

func NewBuildResourceInstance(
	resourceTypeName ResourceType,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	buildID int,
	pipelineID int,
	resourceTypes atc.ResourceTypes,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
) ResourceInstance {
	return &buildResourceInstance{
		resourceInstance: resourceInstance{
			resourceTypeName: resourceTypeName,
			version:          version,
			source:           source,
			params:           params,
		},
		buildID:                buildID,
		pipelineID:             pipelineID,
		resourceTypes:          resourceTypes,
		dbResourceCacheFactory: dbResourceCacheFactory,
	}
}

func (bri buildResourceInstance) FindOrCreateOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, error) {
	resourceCache, err := bri.dbResourceCacheFactory.FindOrCreateResourceCacheForBuild(
		logger,
		bri.buildID,
		string(bri.resourceTypeName),
		bri.version,
		bri.source,
		bri.params,
		bri.pipelineID,
		bri.resourceTypes,
	)
	if err != nil {
		return nil, err
	}

	return workerClient.FindOrCreateVolumeForResourceCache(
		logger,
		worker.VolumeSpec{
			Strategy: worker.ResourceCacheStrategy{
				ResourceHash:    GenerateResourceHash(bri.source, string(bri.resourceTypeName)),
				ResourceVersion: bri.version,
			},
			Properties: bri.volumeProperties(),
			Privileged: true,
		},
		resourceCache,
	)
}

func (bri buildResourceInstance) FindOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, bool, error) {
	resourceCache, err := bri.dbResourceCacheFactory.FindOrCreateResourceCacheForBuild(
		logger,
		bri.buildID,
		string(bri.resourceTypeName),
		bri.version,
		bri.source,
		bri.params,
		bri.pipelineID,
		bri.resourceTypes,
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

type resourceResourceInstance struct {
	resourceInstance
	resourceID             int
	pipelineID             int
	resourceTypes          atc.ResourceTypes
	dbResourceCacheFactory dbng.ResourceCacheFactory
}

func NewResourceResourceInstance(
	resourceTypeName ResourceType,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	resourceID int,
	pipelineID int,
	resourceTypes atc.ResourceTypes,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
) ResourceInstance {
	return &resourceResourceInstance{
		resourceInstance: resourceInstance{
			resourceTypeName: resourceTypeName,
			version:          version,
			source:           source,
			params:           params,
		},
		resourceID:             resourceID,
		pipelineID:             pipelineID,
		resourceTypes:          resourceTypes,
		dbResourceCacheFactory: dbResourceCacheFactory,
	}
}

func (rri resourceResourceInstance) FindOrCreateOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, error) {
	resourceCache, err := rri.dbResourceCacheFactory.FindOrCreateResourceCacheForResource(
		logger,
		rri.resourceID,
		string(rri.resourceTypeName),
		rri.version,
		rri.source,
		rri.params,
		rri.pipelineID,
		rri.resourceTypes,
	)
	if err != nil {
		return nil, err
	}

	return workerClient.FindOrCreateVolumeForResourceCache(
		logger,
		worker.VolumeSpec{
			Strategy: worker.ResourceCacheStrategy{
				ResourceHash:    GenerateResourceHash(rri.source, string(rri.resourceTypeName)),
				ResourceVersion: rri.version,
			},
			Properties: rri.volumeProperties(),
			Privileged: true,
		},
		resourceCache,
	)
}

func (rri resourceResourceInstance) FindOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, bool, error) {
	resourceCache, err := rri.dbResourceCacheFactory.FindOrCreateResourceCacheForResource(
		logger,
		rri.resourceID,
		string(rri.resourceTypeName),
		rri.version,
		rri.source,
		rri.params,
		rri.pipelineID,
		rri.resourceTypes,
	)
	if err != nil {
		logger.Error("failed-to-find-or-initialized-volume-resource-cache-for-resource", err)
		return nil, false, err
	}

	return workerClient.FindInitializedVolumeForResourceCache(
		logger,
		resourceCache,
	)
}

type resourceTypeResourceInstance struct {
	resourceInstance
	resourceType           *dbng.UsedResourceType
	pipelineID             int
	resourceTypes          atc.ResourceTypes
	dbResourceCacheFactory dbng.ResourceCacheFactory
}

func NewResourceTypeResourceInstance(
	resourceTypeName ResourceType,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	resourceType *dbng.UsedResourceType,
	pipelineID int,
	resourceTypes atc.ResourceTypes,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
) ResourceInstance {
	return &resourceTypeResourceInstance{
		resourceInstance: resourceInstance{
			resourceTypeName: resourceTypeName,
			version:          version,
			source:           source,
			params:           params,
		},
		resourceType:           resourceType,
		pipelineID:             pipelineID,
		resourceTypes:          resourceTypes,
		dbResourceCacheFactory: dbResourceCacheFactory,
	}
}

func (rtri resourceTypeResourceInstance) FindOrCreateOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, error) {
	resourceCache, err := rtri.dbResourceCacheFactory.FindOrCreateResourceCacheForResourceType(
		logger,
		string(rtri.resourceTypeName),
		rtri.version,
		rtri.source,
		rtri.params,
		rtri.pipelineID,
		rtri.resourceTypes,
	)
	if err != nil {
		logger.Error("failed-to-find-or-create-resource-cache-for-resource-type", err)
		return nil, err
	}

	return workerClient.FindOrCreateVolumeForResourceCache(
		logger,
		worker.VolumeSpec{
			Strategy: worker.ResourceCacheStrategy{
				ResourceHash:    GenerateResourceHash(rtri.source, string(rtri.resourceTypeName)),
				ResourceVersion: rtri.version,
			},
			Properties: rtri.volumeProperties(),
			Privileged: true,
		},
		resourceCache,
	)
}

func (rtri resourceTypeResourceInstance) FindOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, bool, error) {
	resourceCache, err := rtri.dbResourceCacheFactory.FindOrCreateResourceCacheForResourceType(
		logger,
		string(rtri.resourceTypeName),
		rtri.version,
		rtri.source,
		rtri.params,
		rtri.pipelineID,
		rtri.resourceTypes,
	)
	if err != nil {
		logger.Error("failed-to-find-or-initialized-volume-resource-cache-for-resource-type", err)
		return nil, false, err
	}

	return workerClient.FindInitializedVolumeForResourceCache(
		logger,
		resourceCache,
	)
}

type resourceInstance struct {
	resourceTypeName ResourceType
	version          atc.Version
	source           atc.Source
	params           atc.Params
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
