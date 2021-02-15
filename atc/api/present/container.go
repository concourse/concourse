package present

import (
	"encoding/json"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func Container(container db.Container, expiresAt time.Time) atc.Container {
	meta := container.Metadata()

	var pipelineInstanceVars atc.InstanceVars
	_ = json.Unmarshal([]byte(meta.PipelineInstanceVars), &pipelineInstanceVars)

	atcContainer := atc.Container{
		ID:         container.Handle(),
		WorkerName: container.WorkerName(),

		Type:  string(meta.Type),
		State: container.State(),

		PipelineID: meta.PipelineID,
		JobID:      meta.JobID,
		BuildID:    meta.BuildID,

		PipelineName:         meta.PipelineName,
		PipelineInstanceVars: pipelineInstanceVars,
		JobName:              meta.JobName,
		BuildName:            meta.BuildName,

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
