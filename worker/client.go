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
	CreateContainer(
		lager.Logger,
		<-chan os.Signal,
		ImageFetchingDelegate,
		Identifier,
		Metadata,
		ContainerSpec,
		atc.ResourceTypes,
	) (Container, error)

	FindContainerForIdentifier(lager.Logger, Identifier) (Container, bool, error)
	LookupContainer(lager.Logger, string) (Container, bool, error)
	ValidateResourceCheckVersion(container db.SavedContainer) (bool, error)
	FindResourceTypeByPath(path string) (atc.WorkerResourceType, bool)
	FindVolume(lager.Logger, VolumeSpec) (Volume, bool, error)
	CreateVolume(logger lager.Logger, vs VolumeSpec, teamID int) (Volume, error)
	ListVolumes(lager.Logger, VolumeProperties) ([]Volume, error)
	LookupVolume(lager.Logger, string) (Volume, bool, error)

	Satisfying(WorkerSpec, atc.ResourceTypes) (Worker, error)
	AllSatisfying(WorkerSpec, atc.ResourceTypes) ([]Worker, error)
	RunningWorkers() ([]Worker, error)
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

type ResourceCacheStrategy struct {
	ResourceHash    string
	ResourceVersion atc.Version
}

func (ResourceCacheStrategy) baggageclaimStrategy() baggageclaim.Strategy {
	return baggageclaim.EmptyStrategy{}
}

func (strategy ResourceCacheStrategy) dbIdentifier() db.VolumeIdentifier {
	return db.VolumeIdentifier{
		ResourceCache: &db.ResourceCacheIdentifier{
			ResourceHash:    strategy.ResourceHash,
			ResourceVersion: strategy.ResourceVersion,
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

	Volumes() []Volume
	VolumeMounts() []VolumeMount

	WorkerName() string
}

type VolumeProperties map[string]string
type VolumeIdentifier db.VolumeIdentifier

type Identifier db.ContainerIdentifier

type Metadata db.ContainerMetadata

func (m Metadata) IsForResource() bool {
	return m.Type == db.ContainerTypeCheck ||
		m.Type == db.ContainerTypeGet ||
		m.Type == db.ContainerTypePut
}
