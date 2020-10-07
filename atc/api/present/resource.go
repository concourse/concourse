package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func Resource(resource db.Resource, showCheckError bool, teamName string) atc.Resource {
	var checkErrString, rcCheckErrString string
	var failingToCheck bool
	if resource.CheckSetupError() != nil && showCheckError {
		checkErrString = resource.CheckSetupError().Error()
	}

	if resource.CheckError() != nil && showCheckError {
		rcCheckErrString = resource.CheckError().Error()
	}

	if resource.CheckSetupError() != nil || resource.CheckError() != nil {
		failingToCheck = true
	}

	atcResource := atc.Resource{
		Name:                 resource.Name(),
		PipelineID:           resource.PipelineID(),
		PipelineName:         resource.PipelineName(),
		PipelineInstanceVars: resource.PipelineInstanceVars(),
		TeamName:             teamName,
		Type:                 resource.Type(),
		Icon:                 resource.Icon(),

		FailingToCheck:  failingToCheck,
		CheckSetupError: checkErrString,
		CheckError:      rcCheckErrString,
		PinComment:      resource.PinComment(),
	}

	if !resource.LastCheckEndTime().IsZero() {
		atcResource.LastChecked = resource.LastCheckEndTime().Unix()
	}

	if resource.ConfigPinnedVersion() != nil {
		atcResource.PinnedVersion = resource.ConfigPinnedVersion()
		atcResource.PinnedInConfig = true
	} else if resource.APIPinnedVersion() != nil {
		atcResource.PinnedVersion = resource.APIPinnedVersion()
		atcResource.PinnedInConfig = false
	}

	return atcResource
}
