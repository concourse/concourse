package present

import "github.com/concourse/atc"

func Resource(resource atc.ResourceConfig, groups atc.GroupConfigs) atc.Resource {
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
	}
}
