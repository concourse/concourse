package resource

import (
	"context"

	"github.com/concourse/concourse/atc/resource/v1"
	"github.com/concourse/concourse/atc/resource/v2"
	"github.com/concourse/concourse/atc/worker"
)

// XXX: better name?
type UnversionedResource interface {
	Info(context.Context) (v2.ResourceInfo, error)
}

type unversionedResource struct {
	container worker.Container
}

func NewUnversionedResource(container worker.Container) UnversionedResource {
	return &unversionedResource{
		container: container,
	}
}

func (resource *unversionedResource) Info(ctx context.Context) (v2.ResourceInfo, error) {
	var info v2.ResourceInfo
	err := v1.RunScript(
		ctx,
		"/info",
		nil,
		nil,
		&info,
		nil,
		false,
		resource.container,
	)
	if err != nil {
		return v2.ResourceInfo{}, err
	}

	return info, nil
}
