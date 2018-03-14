package exec

import (
	"context"
	"errors"
	"io"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

type UserArtifactStep struct {
	id   atc.PlanID
	name worker.ArtifactName
}

func UserArtifact(id atc.PlanID, name worker.ArtifactName) Step {
	return &UserArtifactStep{
		id:   id,
		name: name,
	}
}

func (step *UserArtifactStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx).WithData(lager.Data{
		"plan-id": step.id,
		"name":    step.name,
	})

	state.Artifacts().RegisterSource(step.name, streamSource{
		logger,
		step.id,
		state,
	})

	return nil
}

func (step *UserArtifactStep) Succeeded() bool {
	return true
}

type streamSource struct {
	logger lager.Logger
	id     atc.PlanID
	state  RunState
}

func (source streamSource) StreamTo(dest worker.ArtifactDestination) error {
	return source.state.ReadUserInput(source.id, func(rc io.ReadCloser) error {
		source.logger.Debug("reading-user-input")
		return dest.StreamIn(".", rc)
	})
}

func (source streamSource) StreamFile(path string) (io.ReadCloser, error) {
	return nil, errors.New("cannot stream single file from user artifact")
}

func (source streamSource) VolumeOn(worker.Worker) (worker.Volume, bool, error) {
	return nil, false, nil
}
