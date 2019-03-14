package image

import (
	"github.com/concourse/concourse/atc"
)

type getEventHandler struct{}

func NewGetEventHandler() *getEventHandler {
	return &getEventHandler{}
}

// Image resources do not need to save metadata because we do not save image
// resource versions into the database
func (g *getEventHandler) SaveMetadata(metadata atc.Metadata) error {
	return nil
}
