package resource

import (
	"errors"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

type ResourceType string
type ContainerImage string

type Session struct {
	ID        worker.Identifier
	Ephemeral bool
}

//go:generate counterfeiter . Tracker

type Tracker interface {
	Init(Metadata, Session, ResourceType, atc.Tags) (Resource, error)
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

func NewTracker(workerClient worker.Client) Tracker {
	return &tracker{
		workerClient: workerClient,
	}
}

func (tracker *tracker) Init(metadata Metadata, session Session, typ ResourceType, tags atc.Tags) (Resource, error) {
	container, err := tracker.workerClient.FindContainerForIdentifier(session.ID)

	switch err {
	case nil:
	case worker.ErrContainerNotFound:
		container, err = tracker.workerClient.CreateContainer(session.ID, worker.ResourceTypeContainerSpec{
			Type:      string(typ),
			Ephemeral: session.Ephemeral,
			Tags:      tags,
			Env:       metadata.Env(),
		})
	}

	if err != nil {
		return nil, err
	}

	return NewResource(container, typ), nil
}
