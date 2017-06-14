package worker

import (
	"os"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/baggageclaim"
)

//go:generate counterfeiter . Client

type Client interface {
	FindOrCreateContainer(
		lager.Logger,
		<-chan os.Signal,
		ImageFetchingDelegate,
		db.ResourceUser,
		db.ContainerOwner,
		db.ContainerMetadata,
		ContainerSpec,
		atc.VersionedResourceTypes,
	) (Container, error)

	FindVolumeForResourceCache(
		logger lager.Logger,
		resourceCache *db.UsedResourceCache,
	) (Volume, bool, error)

	FindContainerByHandle(lager.Logger, int, string) (Container, bool, error)
	FindResourceTypeByPath(path string) (atc.WorkerResourceType, bool)
	LookupVolume(lager.Logger, string) (Volume, bool, error)

	Satisfying(lager.Logger, WorkerSpec, atc.VersionedResourceTypes) (Worker, error)
	AllSatisfying(lager.Logger, WorkerSpec, atc.VersionedResourceTypes) ([]Worker, error)
	RunningWorkers(lager.Logger) ([]Worker, error)
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

type VolumeProperties map[string]string
