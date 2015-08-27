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
	Init(Session, ResourceType, atc.Tags) (Resource, error)
}

type tracker struct {
	workerClient worker.Client
}

var ErrUnknownResourceType = errors.New("unknown resource type")

func NewTracker(workerClient worker.Client) Tracker {
	return &tracker{
		workerClient: workerClient,
	}
}

func (tracker *tracker) Init(session Session, typ ResourceType, tags atc.Tags) (Resource, error) {
	container, err := tracker.workerClient.FindContainerForIdentifier(session.ID)

	switch err {
	case nil:
	case worker.ErrContainerNotFound:
		container, err = tracker.workerClient.CreateContainer(session.ID, worker.ResourceTypeContainerSpec{
			Type:      string(typ),
			Ephemeral: session.Ephemeral,
			Tags:      tags,
		})
	}

	if err != nil {
		return nil, err
	}

	return NewResource(container, typ), nil
}
