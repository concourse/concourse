package resource

import (
	"crypto/sha512"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/worker"
)

const reapExtraVolumeTTL = time.Minute

var ErrResourceTypeNotFound = errors.New("resource type not found")

//go:generate counterfeiter . ResourceInstance

type ResourceInstance interface {
	FindOn(lager.Logger, worker.Client) (worker.Volume, bool, error)
	CreateOn(lager.Logger, worker.Client) (worker.Volume, error)

	VolumeIdentifier() worker.VolumeIdentifier
}

type buildResourceInstance struct {
	resourceInstance
	build                  *dbng.Build
	pipeline               *dbng.Pipeline
	resourceTypes          atc.ResourceTypes
	dbResourceCacheFactory dbng.ResourceCacheFactory
}

func NewBuildResourceInstance(
	resourceTypeName ResourceType,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	build *dbng.Build,
	pipeline *dbng.Pipeline,
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
		build:                  build,
		pipeline:               pipeline,
		resourceTypes:          resourceTypes,
		dbResourceCacheFactory: dbResourceCacheFactory,
	}
}

func (bri buildResourceInstance) CreateOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, error) {
	resourceCache, err := bri.dbResourceCacheFactory.FindOrCreateResourceCacheForBuild(
		bri.build,
		string(bri.resourceTypeName),
		bri.version,
		bri.source,
		bri.params,
		bri.pipeline,
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
			TTL:        0,
		},
		resourceCache,
	)
}

type resourceResourceInstance struct {
	resourceInstance
	resource               *dbng.Resource
	pipeline               *dbng.Pipeline
	resourceTypes          atc.ResourceTypes
	dbResourceCacheFactory dbng.ResourceCacheFactory
}

func NewResourceResourceInstance(
	resourceTypeName ResourceType,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	resource *dbng.Resource,
	pipeline *dbng.Pipeline,
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
		resource:               resource,
		pipeline:               pipeline,
		resourceTypes:          resourceTypes,
		dbResourceCacheFactory: dbResourceCacheFactory,
	}
}

func (rri resourceResourceInstance) CreateOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, error) {
	resourceCache, err := rri.dbResourceCacheFactory.FindOrCreateResourceCacheForResource(
		rri.resource,
		string(rri.resourceTypeName),
		rri.version,
		rri.source,
		rri.params,
		rri.pipeline,
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
			TTL:        0,
		},
		resourceCache,
	)
}

type resourceTypeResourceInstance struct {
	resourceInstance
	resourceType           *dbng.UsedResourceType
	pipeline               *dbng.Pipeline
	resourceTypes          atc.ResourceTypes
	dbResourceCacheFactory dbng.ResourceCacheFactory
}

func NewResourceTypeResourceInstance(
	resourceTypeName ResourceType,
	version atc.Version,
	source atc.Source,
	params atc.Params,
	resourceType *dbng.UsedResourceType,
	pipeline *dbng.Pipeline,
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
		pipeline:               pipeline,
		resourceTypes:          resourceTypes,
		dbResourceCacheFactory: dbResourceCacheFactory,
	}
}

func (rtri resourceTypeResourceInstance) CreateOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, error) {
	resourceCache, err := rtri.dbResourceCacheFactory.FindOrCreateResourceCacheForResourceType(
		string(rtri.resourceTypeName),
		rtri.version,
		rtri.source,
		rtri.params,
		rtri.pipeline,
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
			TTL:        0,
		},
		resourceCache,
	)
}

type resourceInstance struct {
	resourceTypeName ResourceType
	version          atc.Version
	source           atc.Source
	params           atc.Params
}

func (instance resourceInstance) FindOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, bool, error) {
	volumes, err := workerClient.ListVolumes(logger, instance.initializedVolumeProperties())
	if err != nil {
		logger.Error("failed-to-list-volumes", err)
		return nil, false, err
	}

	switch len(volumes) {
	case 0:
		logger.Debug("no-volumes-found")
		return nil, false, nil
	case 1:
		return volumes[0], true, nil
	default:
		return selectLowestAlphabeticalVolume(logger, volumes), true, nil
	}
}

func (instance resourceInstance) CreateOn(logger lager.Logger, workerClient worker.Client) (worker.Volume, error) {
	return nil, errors.New("CreateOn not implemented for resourceInstance")
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

func (instance resourceInstance) initializedVolumeProperties() worker.VolumeProperties {
	props := instance.volumeProperties()
	props["initialized"] = "yep"
	return props
}

func (instance resourceInstance) VolumeIdentifier() worker.VolumeIdentifier {
	return worker.VolumeIdentifier{
		ResourceCache: &db.ResourceCacheIdentifier{
			ResourceVersion: instance.version,
			ResourceHash:    GenerateResourceHash(instance.source, string(instance.resourceTypeName)),
		},
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
