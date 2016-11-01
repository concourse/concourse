package worker

import (
	"os"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/baggageclaim"
)

//go:generate counterfeiter . Client

type Client interface {
	CreateBuildContainer(
		lager.Logger,
		<-chan os.Signal,
		ImageFetchingDelegate,
		Identifier,
		Metadata,
		ContainerSpec,
		atc.ResourceTypes,
		map[string]string,
	) (Container, error)

	CreateResourceGetContainer(
		logger lager.Logger,
		cancel <-chan os.Signal,
		delegate ImageFetchingDelegate,
		id Identifier,
		metadata Metadata,
		spec ContainerSpec,
		resourceTypes atc.ResourceTypes,
		outputPaths map[string]string,
		resourceType string,
		version atc.Version,
		source atc.Source,
		params atc.Params,
	) (Container, error)

	CreateResourceCheckContainer(
		logger lager.Logger,
		cancel <-chan os.Signal,
		delegate ImageFetchingDelegate,
		id Identifier,
		metadata Metadata,
		spec ContainerSpec,
		resourceTypes atc.ResourceTypes,
		resourceType string,
		source atc.Source,
	) (Container, error)

	CreateResourceTypeCheckContainer(
		logger lager.Logger,
		cancel <-chan os.Signal,
		delegate ImageFetchingDelegate,
		id Identifier,
		metadata Metadata,
		spec ContainerSpec,
		resourceTypes atc.ResourceTypes,
		resourceType string,
		source atc.Source,
	) (Container, error)

	FindOrCreateContainerForIdentifier(
		logger lager.Logger,
		id Identifier,
		metadata Metadata,
		containerSpec ContainerSpec,
		resourceTypes atc.ResourceTypes,
		imageFetchingDelegate ImageFetchingDelegate,
		resourceSources map[string]ArtifactSource,
	) (Container, []string, error)

	FindContainerForIdentifier(lager.Logger, Identifier) (Container, bool, error)
	LookupContainer(lager.Logger, string) (Container, bool, error)
	ValidateResourceCheckVersion(container db.SavedContainer) (bool, error)
	FindResourceTypeByPath(path string) (atc.WorkerResourceType, bool)
	FindVolume(lager.Logger, VolumeSpec) (Volume, bool, error)
	CreateVolumeForResourceCache(logger lager.Logger, vs VolumeSpec) (Volume, error)
	ListVolumes(lager.Logger, VolumeProperties) ([]Volume, error)
	LookupVolume(lager.Logger, string) (Volume, bool, error)

	Satisfying(WorkerSpec, atc.ResourceTypes) (Worker, error)
	AllSatisfying(WorkerSpec, atc.ResourceTypes) ([]Worker, error)
	Workers() ([]Worker, error)
	GetWorker(workerName string) (Worker, error)
}

type VolumeSpec struct {
	Strategy   Strategy
	Properties VolumeProperties
	Privileged bool
	TTL        time.Duration
}

func (spec VolumeSpec) baggageclaimVolumeSpec() baggageclaim.VolumeSpec {
	return baggageclaim.VolumeSpec{
		Strategy:   spec.Strategy.baggageclaimStrategy(),
		Privileged: spec.Privileged,
		Properties: baggageclaim.VolumeProperties(spec.Properties),
		TTL:        spec.TTL,
	}
}

type Strategy interface {
	baggageclaimStrategy() baggageclaim.Strategy
	dbIdentifier() db.VolumeIdentifier
}

type buildResourceCacheStrategy struct {
	resourceCacheStrategy
	build *dbng.Build
}

func NewBuildResourceCacheStrategy(
	resourceHash string,
	resourceVersion atc.Version,
	build *dbng.Build,
) Strategy {
	return buildResourceCacheStrategy{
		resourceCacheStrategy: resourceCacheStrategy{
			resourceHash:    resourceHash,
			resourceVersion: resourceVersion,
		},
		build: build,
	}
}

type resourceResourceCacheStrategy struct {
	resourceCacheStrategy
	resource *dbng.Resource
}

func NewResourceResourceCacheStrategy(
	resourceHash string,
	resourceVersion atc.Version,
	resource *dbng.Resource,
) Strategy {
	return resourceResourceCacheStrategy{
		resourceCacheStrategy: resourceCacheStrategy{
			resourceHash:    resourceHash,
			resourceVersion: resourceVersion,
		},
		resource: resource,
	}
}

type resourceTypeResourceCacheStrategy struct {
	resourceCacheStrategy
	resourceType *dbng.UsedResourceType
}

func NewResourceTypeResourceCacheStrategy(
	resourceHash string,
	resourceVersion atc.Version,
	resourceType *dbng.UsedResourceType,
) Strategy {
	return resourceTypeResourceCacheStrategy{
		resourceCacheStrategy: resourceCacheStrategy{
			resourceHash:    resourceHash,
			resourceVersion: resourceVersion,
		},
		resourceType: resourceType,
	}
}

type resourceCacheStrategy struct {
	resourceHash    string
	resourceVersion atc.Version
}

func (resourceCacheStrategy) baggageclaimStrategy() baggageclaim.Strategy {
	return baggageclaim.EmptyStrategy{}
}

func (strategy resourceCacheStrategy) dbIdentifier() db.VolumeIdentifier {
	return db.VolumeIdentifier{
		ResourceCache: &db.ResourceCacheIdentifier{
			ResourceHash:    strategy.resourceHash,
			ResourceVersion: strategy.resourceVersion,
		},
	}
}

type OutputStrategy struct {
	Name string
}

func (OutputStrategy) baggageclaimStrategy() baggageclaim.Strategy {
	return baggageclaim.EmptyStrategy{}
}

func (strategy OutputStrategy) dbIdentifier() db.VolumeIdentifier {
	return db.VolumeIdentifier{
		Output: &db.OutputIdentifier{
			Name: strategy.Name,
		},
	}
}

type ImageArtifactReplicationStrategy struct {
	Name string
}

func (ImageArtifactReplicationStrategy) baggageclaimStrategy() baggageclaim.Strategy {
	return baggageclaim.EmptyStrategy{}
}

func (strategy ImageArtifactReplicationStrategy) dbIdentifier() db.VolumeIdentifier {
	return db.VolumeIdentifier{
		Replication: &db.ReplicationIdentifier{
			ReplicatedVolumeHandle: strategy.Name,
		},
	}
}

type ContainerRootFSStrategy struct {
	Parent Volume
}

func (strategy ContainerRootFSStrategy) baggageclaimStrategy() baggageclaim.Strategy {
	return baggageclaim.COWStrategy{
		Parent: strategy.Parent,
	}
}

func (strategy ContainerRootFSStrategy) dbIdentifier() db.VolumeIdentifier {
	return db.VolumeIdentifier{
		COW: &db.COWIdentifier{
			ParentVolumeHandle: strategy.Parent.Handle(),
		},
	}
}

type HostRootFSStrategy struct {
	Path       string
	WorkerName string
	Version    *string
}

func (strategy HostRootFSStrategy) baggageclaimStrategy() baggageclaim.Strategy {
	return baggageclaim.ImportStrategy{
		Path: strategy.Path,
	}
}

func (strategy HostRootFSStrategy) dbIdentifier() db.VolumeIdentifier {
	return db.VolumeIdentifier{
		Import: &db.ImportIdentifier{
			Path:       strategy.Path,
			WorkerName: strategy.WorkerName,
			Version:    strategy.Version,
		},
	}
}

//go:generate counterfeiter . Container

type Container interface {
	garden.Container

	Destroy() error

	Release(*time.Duration)

	VolumeMounts() []VolumeMount

	WorkerName() string
}

type VolumeProperties map[string]string
type VolumeIdentifier db.VolumeIdentifier

type Identifier db.ContainerIdentifier
type Metadata db.ContainerMetadata
