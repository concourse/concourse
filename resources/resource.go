package resources

import (
	"encoding/json"
	"fmt"

	"github.com/winston-ci/prole/api/builds"
)

type Resource struct {
	Name string

	Type string
	URI  string
}

func (resource Resource) BuildInput() builds.Input {
	version := json.RawMessage(fmt.Sprintf(`{"uri":%q}`, resource.URI))

	return builds.Input{
		Type: resource.Type,

		DestinationPath: resource.Name,

		Version: &version,
	}
}
