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
	id       atc.PlanID
	name     worker.ArtifactName
	delegate BuildStepDelegate
}

func ArtifactOutput(id atc.PlanID, name worker.ArtifactName, delegate BuildStepDelegate) Step {
	return &ArtifactOutputStep{
		id:       id,
		name:     name,
		delegate: delegate,
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

	pb := progress(string(step.name)+":", step.delegate.Stdout())

	return state.SendPlanOutput(step.id, func(w io.Writer) error {
		pb.Start()
		defer pb.Finish()

		logger.Debug("sending-plan-output")
		return source.StreamTo(streamDestination{io.MultiWriter(w, pb)})
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
