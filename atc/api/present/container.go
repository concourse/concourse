package present

import (
	"time"

	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/db"
)

func Container(container db.Container, expiresAt time.Time) atc.Container {
	meta := container.Metadata()

	atcContainer := atc.Container{
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

	if !expiresAt.IsZero() {
		atcContainer.ExpiresIn = expiresAt.Sub(time.Now()).Round(time.Second).String()
	}

	return atcContainer
}
