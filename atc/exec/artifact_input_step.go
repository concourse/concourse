package exec

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/build"
)

type ArtifactVolumeNotFoundError struct {
	ArtifactName string
}

func (e ArtifactVolumeNotFoundError) Error() string {
	return fmt.Sprintf("volume for worker artifact '%s' not found", e.ArtifactName)
}

type ArtifactInputStep struct {
	plan       atc.Plan
	build      db.Build
	workerPool Pool
}

func NewArtifactInputStep(plan atc.Plan, build db.Build, workerPool Pool) Step {
	return &ArtifactInputStep{
		plan:       plan,
		build:      build,
		workerPool: workerPool,
	}
}

func (step *ArtifactInputStep) Run(ctx context.Context, state RunState) (bool, error) {
	logger := lagerctx.FromContext(ctx).WithData(lager.Data{
		"plan-id": step.plan.ID,
	})

	buildArtifact, err := step.build.Artifact(step.plan.ArtifactInput.ArtifactID)
	if err != nil {
		return false, err
	}

	// TODO (runtime/#3607): artifact_input_step shouldn't know about db Volumem
	//		has a runState with artifact repo. We could use that.
	createdVolume, found, err := buildArtifact.Volume(step.build.TeamID())
	if err != nil {
		return false, err
	}

	if !found {
		return false, ArtifactVolumeNotFoundError{buildArtifact.Name()}
	}

	// TODO (runtime/#3607): artifact_input_step shouldn't be looking up the volume on the worker
	volume, _, found, err := step.workerPool.LocateVolume(logger, createdVolume.TeamID(), createdVolume.Handle())
	if err != nil {
		return false, err
	}

	if !found {
		return false, ArtifactVolumeNotFoundError{buildArtifact.Name()}
	}

	logger.Info("register-artifact-source", lager.Data{
		"artifact_id": buildArtifact.ID(),
		"handle":      volume.Handle(),
	})

	state.ArtifactRepository().RegisterArtifact(build.ArtifactName(step.plan.ArtifactInput.Name), volume)

	return true, nil
}
