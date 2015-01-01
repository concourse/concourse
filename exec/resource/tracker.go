package resource

import (
	"errors"

	garden "github.com/cloudfoundry-incubator/garden/api"
)

type ResourceMapping map[ResourceType]ContainerImage
type ResourceType string
type ContainerImage string

//go:generate counterfeiter . Tracker
type Tracker interface {
	Init(ResourceType) (Resource, error)
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

func (tracker *tracker) Init(typ ResourceType) (Resource, error) {
	resourceImage, found := tracker.resourceTypes[typ]
	if !found {
		return nil, ErrUnknownResourceType
	}

	container, err := tracker.gardenClient.Create(garden.ContainerSpec{
		RootFSPath: string(resourceImage),
		Privileged: true,
	})
	if err != nil {
		return nil, err
	}

	return NewResource(container, tracker.gardenClient), nil
}
