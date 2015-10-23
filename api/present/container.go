package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func Container(container db.Container) atc.Container {
	return atc.Container{
		ID:           container.Handle,
		PipelineName: container.PipelineName,
		Type:         container.Type.String(),
		Name:         container.Name,
		BuildID:      container.BuildID,
		WorkerName:   container.WorkerName,
	}
}
