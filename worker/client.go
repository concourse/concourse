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
	FindOrCreateBuildContainer(
		lager.Logger,
		<-chan os.Signal,
		ImageFetchingDelegate,
		int,
		atc.PlanID,
		dbng.ContainerMetadata,
		ContainerSpec,
		atc.VersionedResourceTypes,
	) (Container, error)

	CreateResourceGetContainer(
		logger lager.Logger,
		resourceUser dbng.ResourceUser,
		cancel <-chan os.Signal,
		delegate ImageFetchingDelegate,
		metadata dbng.ContainerMetadata,
		spec ContainerSpec,
		resourceTypes atc.VersionedResourceTypes,
		resourceType string,
		version atc.Version,
		source atc.Source,
		params atc.Params,
	) (Container, error)

	FindOrCreateResourceCheckContainer(
		logger lager.Logger,
		resourceUser dbng.ResourceUser,
		cancel <-chan os.Signal,
		delegate ImageFetchingDelegate,
		metadata dbng.ContainerMetadata,
		spec ContainerSpec,
		resourceTypes atc.VersionedResourceTypes,
		resourceType string,
		source atc.Source,
	) (Container, error)

	CreateVolumeForResourceCache(
		logger lager.Logger,
		vs VolumeSpec,
		resourceCache *dbng.UsedResourceCache,
	) (Volume, error)

	FindInitializedVolumeForResourceCache(
		logger lager.Logger,
		resourceCache *dbng.UsedResourceCache,
	) (Volume, bool, error)

	FindContainerByHandle(lager.Logger, int, string) (Container, bool, error)
	FindResourceTypeByPath(path string) (atc.WorkerResourceType, bool)
	LookupVolume(lager.Logger, string) (Volume, bool, error)

	Satisfying(WorkerSpec, atc.VersionedResourceTypes) (Worker, error)
	AllSatisfying(WorkerSpec, atc.VersionedResourceTypes) ([]Worker, error)
	RunningWorkers() ([]Worker, error)
	GetWorker(workerName string) (Worker, error)
}

//go:generate counterfeiter . InputSource

type InputSource interface {
	Name() ArtifactName
	Source() ArtifactSource
	DestinationPath() string
}

type VolumeSpec struct {
	Strategy   baggageclaim.Strategy
	Properties VolumeProperties
	Privileged bool
	TTL        time.Duration
}

func (spec VolumeSpec) baggageclaimVolumeSpec() baggageclaim.VolumeSpec {
	return baggageclaim.VolumeSpec{
		Strategy:   spec.Strategy,
		Privileged: spec.Privileged,
		Properties: baggageclaim.VolumeProperties(spec.Properties),
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

//go:generate counterfeiter . Container

type Container interface {
	garden.Container

	Destroy() error

	VolumeMounts() []VolumeMount

	WorkerName() string

	MarkAsHijacked() error
}

type ResourceCacheIdentifier db.ResourceCacheIdentifier
type VolumeProperties map[string]string
