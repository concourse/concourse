package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func Resource(resource db.Resource, showCheckError bool, teamName string) atc.Resource {
	var checkErrString, rcCheckErrString string
	var failingToCheck bool
	if resource.CheckError() != nil && showCheckError {
		checkErrString = resource.CheckError().Error()
	}

	if resource.ResourceConfigCheckError() != nil && showCheckError {
		rcCheckErrString = resource.ResourceConfigCheckError().Error()
	}

	if resource.CheckError() != nil || resource.ResourceConfigCheckError() != nil {
		failingToCheck = true
	}

	atcResource := atc.Resource{
		Name:         resource.Name(),
		PipelineName: resource.PipelineName(),
		TeamName:     teamName,
		Type:         resource.Type(),

		Paused: resource.Paused(),

		FailingToCheck:  failingToCheck,
		CheckSetupError: checkErrString,
		CheckError:      rcCheckErrString,
	}

	if !resource.LastChecked().IsZero() {
		atcResource.LastChecked = resource.LastChecked().Unix()
	}

	return atcResource
}
