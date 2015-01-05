package resource

import (
	"errors"

	garden "github.com/cloudfoundry-incubator/garden/api"
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
	gardenClient  garden.Client
}

var ErrUnknownResourceType = errors.New("unknown resource type")

func NewTracker(resourceTypes ResourceMapping, gardenClient garden.Client) Tracker {
	return &tracker{
		resourceTypes: resourceTypes,
		gardenClient:  gardenClient,
	}
}

func (tracker *tracker) Init(sessionID SessionID, typ ResourceType) (Resource, error) {
	container, err := tracker.gardenClient.Lookup(string(sessionID))
	if err != nil {
		resourceImage, found := tracker.resourceTypes[typ]
		if !found {
			return nil, ErrUnknownResourceType
		}

		container, err = tracker.gardenClient.Create(garden.ContainerSpec{
			Handle:     string(sessionID),
			RootFSPath: string(resourceImage),
			Privileged: true,
		})
		if err != nil {
			return nil, err
		}
	}

	return NewResource(container, tracker.gardenClient), nil
}
