package worker

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
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

	FindResourceTypeByPath(path string) (atc.WorkerResourceType, bool)
	FindVolume(lager.Logger, VolumeSpec) (Volume, bool, error)
	CreateVolume(logger lager.Logger, vs VolumeSpec, teamID int) (Volume, error)
	ListVolumes(lager.Logger, VolumeProperties) ([]Volume, error)
	LookupVolume(lager.Logger, string) (Volume, bool, error)

	Satisfying(WorkerSpec, atc.ResourceTypes) (Worker, error)
	AllSatisfying(WorkerSpec, atc.ResourceTypes) ([]Worker, error)
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

type MultipleWorkersFoundContainerError struct {
	Names []string
}

func (err MultipleWorkersFoundContainerError) Error() string {
	return fmt.Sprintf("multiple workers found specified container, expected one: %s", strings.Join(err.Names, ", "))
}

type MultipleContainersError struct {
	Handles []string
}

func (err MultipleContainersError) Error() string {
	return fmt.Sprintf("multiple containers found, expected one: %s", strings.Join(err.Handles, ", "))
}

type MultiWorkerError struct {
	workerErrors map[string]error
}

func (mwe *MultiWorkerError) AddError(workerName string, err error) {
	if mwe.workerErrors == nil {
		mwe.workerErrors = map[string]error{}
	}

	mwe.workerErrors[workerName] = err
}

func (mwe MultiWorkerError) Errors() map[string]error {
	return mwe.workerErrors
}

func (err MultiWorkerError) Error() string {
	errorMessage := ""
	if err.workerErrors != nil {
		for workerName, err := range err.workerErrors {
			errorMessage = fmt.Sprintf("%s workerName: %s, error: %s", errorMessage, workerName, err)
		}
	}
	return errorMessage
}
