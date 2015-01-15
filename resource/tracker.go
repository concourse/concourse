package resource

import (
	"errors"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc/worker"
)

type ResourceMapping map[ResourceType]ContainerImage
type ResourceType string
type ContainerImage string

type SessionID string

//go:generate counterfeiter . Tracker
type Tracker interface {
	Init(SessionID, ResourceType) (Resource, error)
}

type tracker struct {
	resourceTypes ResourceMapping
	workerClient  worker.Client
}

var ErrUnknownResourceType = errors.New("unknown resource type")

func NewTracker(resourceTypes ResourceMapping, workerClient worker.Client) Tracker {
	return &tracker{
		resourceTypes: resourceTypes,
		workerClient:  workerClient,
	}
}

func (tracker *tracker) Init(sessionID SessionID, typ ResourceType) (Resource, error) {
	container, err := tracker.workerClient.Lookup(string(sessionID))
	if err != nil {
		resourceImage, found := tracker.resourceTypes[typ]
		if !found {
			return nil, ErrUnknownResourceType
		}

		container, err = tracker.workerClient.Create(garden.ContainerSpec{
			Handle:     string(sessionID),
			RootFSPath: string(resourceImage),
			Privileged: true,
		})
		if err != nil {
			return nil, err
		}
	}

	return NewResource(container, typ), nil
}
