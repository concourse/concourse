package resource

import (
	"errors"

	"github.com/concourse/atc/worker"
)

type ResourceType string
type ContainerImage string

type Session struct {
	ID string
}

//go:generate counterfeiter . Tracker

type Tracker interface {
	Init(Session, ResourceType) (Resource, error)
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

func (tracker *tracker) Init(session Session, typ ResourceType) (Resource, error) {
	container, err := tracker.workerClient.Lookup(string(session.ID))
	if err != nil {
		container, err = tracker.workerClient.CreateContainer(string(session.ID), worker.ResourceTypeContainerSpec{
			Type: string(typ),
		})
		if err != nil {
			return nil, err
		}
	}

	return NewResource(container, typ), nil
}
