package present

import "github.com/concourse/atc"

func Resource(resource atc.ResourceConfig) atc.Resource {
	return atc.Resource{
		Name: resource.Name,
		Type: resource.Type,
	}
}
