package exec

import (
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

type ErrPipelineNotFound struct {
	PipelineName string
}

func (e ErrPipelineNotFound) Error() string {
	return fmt.Sprintf("pipeline '%s' not found", e.PipelineName)
}

type ErrResourceNotFound struct {
	ResourceName string
}

func (e ErrResourceNotFound) Error() string {
	return fmt.Sprintf("resource '%s' not found", e.ResourceName)
}

type GetEventHandler struct {
	resource db.Resource
	version  atc.Version
	space    atc.Space
}

func NewGetEventHandler(resource db.Resource, space atc.Space, version atc.Version) *GetEventHandler {
	return &GetEventHandler{
		resource: resource,
		space:    space,
		version:  version,
	}
}

func (g *GetEventHandler) SaveMetadata(metadata atc.Metadata) error {
	if g.resource != nil {
		if len(metadata) != 0 {
			err := g.resource.SaveMetadata(g.space, g.version, metadata)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
