package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func Container(container db.Container) atc.Container {
	meta := container.Metadata()

	return atc.Container{
		ID:         container.Handle(),
		WorkerName: container.WorkerName(),

		Type:  string(meta.Type),
		State: container.State(),

		PipelineID: meta.PipelineID,
		JobID:      meta.JobID,
		BuildID:    meta.BuildID,

		PipelineName: meta.PipelineName,
		JobName:      meta.JobName,
		BuildName:    meta.BuildName,

		StepName: meta.StepName,
		Attempt:  meta.Attempt,

		WorkingDirectory: meta.WorkingDirectory,
		User:             meta.User,
	}
}
