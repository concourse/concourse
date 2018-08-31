package worker

import (
	"context"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/baggageclaim"
)

//go:generate counterfeiter . Client

type Client interface {
	FindOrCreateContainer(
		context.Context,
		lager.Logger,
		ImageFetchingDelegate,
		db.ContainerOwner,
		db.ContainerMetadata,
		ContainerSpec,
		creds.VersionedResourceTypes,
	) (Container, error)

	FindContainerByHandle(lager.Logger, int, string) (Container, bool, error)

	LookupVolume(lager.Logger, string) (Volume, bool, error)

	FindResourceTypeByPath(path string) (atc.WorkerResourceType, bool)

	Satisfying(lager.Logger, WorkerSpec, creds.VersionedResourceTypes) (Worker, error)
	AllSatisfying(lager.Logger, WorkerSpec, creds.VersionedResourceTypes) ([]Worker, error)
	RunningWorkers(lager.Logger) ([]Worker, error)
}

//go:generate counterfeiter . InputSource

type InputSource interface {
	Source() ArtifactSource
	DestinationPath() string
}

//go:generate counterfeiter . BindMountSource

type BindMountSource interface {
	VolumeOn(Worker) (garden.BindMount, bool, error)
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

//go:generate counterfeiter . Container

type Container interface {
	garden.Container

	Destroy() error

	VolumeMounts() []VolumeMount

	WorkerName() string

	MarkAsHijacked() error
}

type VolumeProperties map[string]string
