package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func Resource(resource db.Resource, groups atc.GroupConfigs, showCheckError bool, teamName string) atc.Resource {
	groupNames := []string{}
	for _, group := range groups {
		for _, name := range group.Resources {
			if name == resource.Name() {
				groupNames = append(groupNames, group.Name)
			}
		}
	}

	var checkErrString string
	if resource.CheckError() != nil && showCheckError {
		checkErrString = resource.CheckError().Error()
	}

	atcResource := atc.Resource{
		Name:         resource.Name(),
		PipelineName: resource.PipelineName(),
		TeamName:     teamName,
		Type:         resource.Type(),
		Groups:       groupNames,

		Paused: resource.Paused(),

		FailingToCheck: resource.FailingToCheck(),
		CheckError:     checkErrString,
	}

	if !resource.LastChecked().IsZero() {
		atcResource.LastChecked = resource.LastChecked().Unix()
	}

	return atcResource
}
