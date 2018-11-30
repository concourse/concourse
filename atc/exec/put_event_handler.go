package exec

import (
	"fmt"

	"github.com/concourse/concourse/atc"
)

type PutMultipleSpacesError struct {
	FirstSpace  atc.Space
	SecondSpace atc.Space
}

func (e PutMultipleSpacesError) Error() string {
	return fmt.Sprintf("multiple spaces returned in put: %s, %s", e.FirstSpace, e.SecondSpace)
}

type putEventHandler struct{}

func NewPutEventHandler() *putEventHandler {
	return &putEventHandler{}
}

func (p *putEventHandler) CreatedResponse(space atc.Space, version atc.Version, metadata atc.Metadata, spaceVersions []atc.SpaceVersion) ([]atc.SpaceVersion, error) {
	if len(spaceVersions) != 0 && spaceVersions[len(spaceVersions)-1].Space != space {
		return nil, PutMultipleSpacesError{FirstSpace: spaceVersions[len(spaceVersions)-1].Space, SecondSpace: space}
	}

	spaceVersions = append(spaceVersions, atc.SpaceVersion{space, version, metadata})
	return spaceVersions, nil
}
