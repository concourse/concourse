package exec

import (
	"context"
	"fmt"

	"github.com/concourse/concourse/atc/runtime"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/worker"
)

type ArtifactVolumeNotFoundError struct {
	ArtifactName string
}

func (e ArtifactVolumeNotFoundError) Error() string {
	return fmt.Sprintf("volume for worker artifact '%s' not found", e.ArtifactName)
}

type ArtifactInputStep struct {
	plan         atc.Plan
	build        db.Build
	workerClient worker.Client
	delegate     BuildStepDelegate
	succeeded    bool
}

func NewArtifactInputStep(plan atc.Plan, build db.Build, workerClient worker.Client, delegate BuildStepDelegate) Step {
	return &ArtifactInputStep{
		plan:         plan,
		build:        build,
		workerClient: workerClient,
		delegate:     delegate,
	}
}

func (step *ArtifactInputStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx).WithData(lager.Data{
		"plan-id": step.plan.ID,
	})

	buildArtifact, err := step.build.Artifact(step.plan.ArtifactInput.ArtifactID)
	if err != nil {
		return err
	}

	// TODO (runtime/#3607): artifact_input_step shouldn't know about db Volumem
	//		has a runState with artifact repo. We could use that.
	createdVolume, found, err := buildArtifact.Volume(step.build.TeamID())
	if err != nil {
		return err
	}

	if !found {
		return ArtifactVolumeNotFoundError{buildArtifact.Name()}
	}

	// TODO (runtime/#3607): artifact_input_step shouldn't be looking up the volume on the worker
	_, found, err = step.workerClient.FindVolume(logger, createdVolume.TeamID(), createdVolume.Handle())
	if err != nil {
		return err
	}

	if !found {
		return ArtifactVolumeNotFoundError{buildArtifact.Name()}
	}

	art := runtime.TaskArtifact{
		VolumeHandle: createdVolume.Handle(),
	}

	logger.Info("register-artifact-source", lager.Data{
		"artifact_id": buildArtifact.ID(),
		"handle":      art.ID(),
	})

	state.ArtifactRepository().RegisterArtifact(build.ArtifactName(step.plan.ArtifactInput.Name), &art)

	step.succeeded = true

	return nil
}

func (step *ArtifactInputStep) Succeeded() bool {
	return step.succeeded
}
