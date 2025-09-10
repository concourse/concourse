package present

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func Container(container db.Container, expiresAt time.Time) atc.Container {
	meta := container.Metadata()

	var pipelineInstanceVars atc.InstanceVars
	_ = sonic.Unmarshal([]byte(meta.PipelineInstanceVars), &pipelineInstanceVars)

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
		atcContainer.ExpiresIn = time.Until(expiresAt).Round(time.Second).String()
	}

	return atcContainer
}
