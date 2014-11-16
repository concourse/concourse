package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/web/routes"
	"github.com/tedsuo/rata"
)

func Resource(resource atc.ResourceConfig, groups atc.GroupConfigs) atc.Resource {
	generator := rata.NewRequestGenerator("", routes.Routes)

	req, err := generator.CreateRequest(
		routes.GetResource,
		rata.Params{"resource": resource.Name},
		nil,
	)
	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	groupNames := []string{}
	for _, group := range groups {
		for _, name := range group.Resources {
			if name == resource.Name {
				groupNames = append(groupNames, group.Name)
			}
		}
	}

	return atc.Resource{
		Name:   resource.Name,
		Type:   resource.Type,
		Groups: groupNames,
		URL:    req.URL.String(),
		Hidden: resource.Hidden,
	}
}
