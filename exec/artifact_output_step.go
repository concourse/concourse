package exec

import (
	"context"
	"io"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

type ArtifactOutputStep struct {
	id   atc.PlanID
	name worker.ArtifactName
}

func ArtifactOutput(id atc.PlanID, name worker.ArtifactName) Step {
	return &ArtifactOutputStep{
		id:   id,
		name: name,
	}
}

func (step *ArtifactOutputStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx).WithData(lager.Data{
		"plan-id": step.id,
		"source":  step.name,
	})

	source, found := state.Artifacts().SourceFor(step.name)
	if !found {
		return UnknownArtifactSourceError{
			SourceName: step.name,
		}
	}

	return state.SendPlanOutput(step.id, func(w io.Writer) error {
		logger.Debug("sending-plan-output")
		return source.StreamTo(streamDestination{w})
	})
}

func (step *ArtifactOutputStep) Succeeded() bool {
	return true
}

type streamDestination struct {
	w io.Writer
}

func (dest streamDestination) StreamIn(path string, src io.Reader) error {
	_, err := io.Copy(dest.w, src)
	return err
}
