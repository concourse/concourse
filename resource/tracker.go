package resource

import (
	"errors"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"
)

type ResourceType string
type ContainerImage string

type Session struct {
	ID        worker.Identifier
	Ephemeral bool
}

//go:generate counterfeiter . Tracker

type Tracker interface {
	Init(Metadata, Session, ResourceType, atc.Tags, VolumeMount) (Resource, error)
}

type Metadata interface {
	Env() []string
}

type EmptyMetadata struct{}

func (m EmptyMetadata) Env() []string { return nil }

type tracker struct {
	workerClient worker.Client
}

var ErrUnknownResourceType = errors.New("unknown resource type")

type TrackerFactory struct{}

func (TrackerFactory) TrackerFor(client worker.Client) Tracker {
	return NewTracker(client)
}

func NewTracker(workerClient worker.Client) Tracker {
	return &tracker{
		workerClient: workerClient,
	}
}

type VolumeMount struct {
	Volume    baggageclaim.Volume
	MountPath string
}

func (tracker *tracker) Init(metadata Metadata, session Session, typ ResourceType, tags atc.Tags, mount VolumeMount) (Resource, error) {
	container, found, err := tracker.workerClient.FindContainerForIdentifier(session.ID)
	if err != nil {
		return nil, err
	}
	if !found {
		container, err = tracker.workerClient.CreateContainer(session.ID, worker.ResourceTypeContainerSpec{
			Type:      string(typ),
			Ephemeral: session.Ephemeral,
			Tags:      tags,
			Env:       metadata.Env(),
			Volume:    mount.Volume,
			MountPath: mount.MountPath,
		})
		if err != nil {
			return nil, err
		}
	}
	return NewResource(container, typ), nil
}
