package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

func Container(container worker.Container) atc.PresentedContainer {
	identifier := container.IdentifierFromProperties()
	return atc.PresentedContainer{
		ID:           container.Handle(),
		PipelineName: identifier.PipelineName,
		Type:         identifier.Type.ToString(),
		Name:         identifier.Name,
		BuildID:      identifier.BuildID,
	}
}
