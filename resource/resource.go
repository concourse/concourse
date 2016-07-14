package resource

import (
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/clock"
	"github.com/tedsuo/ifrit"
)

//go:generate counterfeiter . Resource

type Resource interface {
	SetContainer(worker.Container)
	Get(worker.Volume, IOConfig, atc.Source, atc.Params, atc.Version) VersionedSource
	Put(IOConfig, atc.Source, atc.Params, ArtifactSource) VersionedSource
	Check(atc.Source, atc.Version) ([]atc.Version, error)

	Release(*time.Duration)

	CacheVolume() (worker.Volume, bool)
}

type IOConfig struct {
	Stdout io.Writer
	Stderr io.Writer
}

//go:generate counterfeiter . ArtifactSource

type ArtifactSource interface {
	StreamTo(ArtifactDestination) error

	// VolumeOn returns a Volume object that contains the artifact from the
	// ArtifactSource which is on a particular Worker. If a volume cannot be found
	// or a volume manager cannot be found on the worker then it will return
	// false.
	VolumeOn(worker.Worker) (worker.Volume, bool, error)
}

//go:generate counterfeiter . ArtifactDestination

type ArtifactDestination interface {
	StreamIn(string, io.Reader) error
}

//go:generate counterfeiter . VersionedSource

type VersionedSource interface {
	ifrit.Runner

	Version() atc.Version
	Metadata() []atc.MetadataField

	StreamOut(string) (io.ReadCloser, error)
	StreamIn(string, io.Reader) error
}

func ResourcesDir(suffix string) string {
	return filepath.Join("/tmp", "build", suffix)
}

type resource struct {
	container worker.Container
	typ       ResourceType
	clock     clock.Clock

	releaseOnce sync.Once

	ScriptFailure bool
}

func NewResource(container worker.Container, clock clock.Clock) Resource {
	return &resource{
		container: container,
		clock:     clock,
	}
}

func (resource *resource) Release(finalTTL *time.Duration) {
	if resource.container != nil {
		resource.container.Release(finalTTL)
	}
}

func (resource *resource) CacheVolume() (worker.Volume, bool) {
	if resource.container == nil {
		return nil, false
	}

	mounts := resource.container.VolumeMounts()

	for _, mount := range mounts {
		if mount.MountPath == ResourcesDir("get") {
			return mount.Volume, true
		}
	}

	return nil, false
}
