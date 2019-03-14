package exec

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

type getEventHandler struct {
	resourceConfig db.ResourceConfig
	version        atc.Version
	space          atc.Space
}

func NewGetEventHandler(resourceConfig db.ResourceConfig, space atc.Space, version atc.Version) *getEventHandler {
	return &getEventHandler{
		resourceConfig: resourceConfig,
		space:          space,
		version:        version,
	}
}

func (g *getEventHandler) SaveMetadata(metadata atc.Metadata) error {
	err := g.resourceConfig.SaveMetadata(space, version, metadata)
	return err
}
