package resources

import (
	"fmt"

	"github.com/winston-ci/prole/api/builds"
)

type Resource struct {
	Name string

	Type string
	URI  string
}

func (resource Resource) BuildInput() builds.Input {
	return builds.Input{
		Type: resource.Type,

		Source: builds.Source(fmt.Sprintf(`{"uri":%q}`, resource.URI)),

		DestinationPath: resource.Name,
	}
}
