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

func (p *putEventHandler) CreatedResponse(space atc.Space, version atc.Version, putResponse *atc.PutResponse) error {
	if putResponse.Space != "" && putResponse.Space != space {
		return PutMultipleSpacesError{FirstSpace: putResponse.Space, SecondSpace: space}
	}

	putResponse.Space = space
	putResponse.CreatedVersions = append(putResponse.CreatedVersions, version)
	return nil
}
