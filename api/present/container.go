package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func Container(container db.ContainerInfo) atc.Container {
	return atc.Container{
		ID:           container.Handle,
		PipelineName: container.PipelineName,
		Type:         container.Type.ToString(),
		Name:         container.Name,
		BuildID:      container.BuildID,
		WorkerName:   container.WorkerName,
	}
}
