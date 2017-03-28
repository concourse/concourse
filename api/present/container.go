package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
)

func Container(container dbng.Container) atc.Container {
	meta := container.Metadata()

	return atc.Container{
		ID:         container.Handle(),
		WorkerName: container.WorkerName(),

		Type: string(meta.Type),

		PipelineID:     meta.PipelineID,
		JobID:          meta.JobID,
		BuildID:        meta.BuildID,
		ResourceID:     meta.ResourceID,
		ResourceTypeID: meta.ResourceTypeID,

		PipelineName:     meta.PipelineName,
		JobName:          meta.JobName,
		BuildName:        meta.BuildName,
		ResourceName:     meta.ResourceName,
		ResourceTypeName: meta.ResourceTypeName,

		StepName: meta.StepName,
		Attempt:  meta.Attempt,

		WorkingDirectory: meta.WorkingDirectory,
		User:             meta.User,
	}
}
