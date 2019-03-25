package resource

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	v2 "github.com/concourse/concourse/atc/resource/v2"
	"github.com/concourse/concourse/atc/worker"
)

type ErrUnknownResourceVersion struct {
	version string
}

func (e ErrUnknownResourceVersion) Error() string {
	return fmt.Sprintf("unknown resource version: %s", e.version)
}

//go:generate counterfeiter . Resource

type Resource interface {
	Get(context.Context, v2.GetEventHandler, worker.Volume, atc.IOConfig, atc.Source, atc.Params, atc.Space, atc.Version) error
	Put(context.Context, v2.PutEventHandler, atc.IOConfig, atc.Source, atc.Params) ([]atc.SpaceVersion, error)
	Check(context.Context, v2.CheckEventHandler, atc.Source, map[atc.Space]atc.Version) error
}

type ResourceType string

type Session struct {
	Metadata db.ContainerMetadata
}

type Metadata interface {
	Env() []string
}

//go:generate counterfeiter . ResourceFactory

type ResourceFactory interface {
	NewResourceForContainer(context.Context, worker.Container) (Resource, error)
}

func NewResourceFactory() ResourceFactory {
	return &resourceFactory{}
}

// TODO: This factory is purely used for testing and faking out the Resource
// object. Please remove asap if possible.
type resourceFactory struct{}

func (rf *resourceFactory) NewResourceForContainer(ctx context.Context, container worker.Container) (Resource, error) {
	var resource Resource

	resourceInfo, err := NewUnversionedResource(container).Info(ctx)
	if err == nil {
		if resourceInfo.Artifacts.APIVersion == "2.0" {
			resource = v2.NewResource(container, resourceInfo)
		} else {
			return nil, ErrUnknownResourceVersion{resourceInfo.Artifacts.APIVersion}
		}
	} else if _, ok := err.(garden.ExecutableNotFoundError); ok {
		resource = v2.NewV1Adapter(container)
	} else if err != nil {
		return nil, err
	}

	return resource, nil
}
