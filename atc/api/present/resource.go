package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func Resource(resource db.Resource) atc.Resource {
	atcResource := atc.Resource{
		Name:                 resource.Name(),
		PipelineID:           resource.PipelineID(),
		PipelineName:         resource.PipelineName(),
		PipelineInstanceVars: resource.PipelineInstanceVars(),
		TeamName:             resource.TeamName(),
		Type:                 resource.Type(),
		Icon:                 resource.Icon(),

		PinComment: resource.PinComment(),

		Build: resource.BuildSummary(),
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
