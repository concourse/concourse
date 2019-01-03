package exec

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/artifact"
	"github.com/concourse/concourse/atc/worker"
)

type ArtifactVolumeNotFoundErr string

func (e ArtifactVolumeNotFoundErr) Error() string {
	return fmt.Sprintf("volume for worker artifact '%s' not found", e)
}

type ArtifactStep struct {
	plan         atc.Plan
	build        db.Build
	workerClient worker.Client
	delegate     BuildStepDelegate
	succeeded    bool
}

func NewArtifactStep(plan atc.Plan, build db.Build, workerClient worker.Client, delegate BuildStepDelegate) Step {
	return &ArtifactStep{
		plan:         plan,
		build:        build,
		workerClient: workerClient,
		delegate:     delegate,
	}
}

func (step *ArtifactStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx).WithData(lager.Data{
		"plan-id": step.plan.ID,
	})

	buildArtifact, err := step.build.Artifact(step.plan.UserArtifact.ArtifactID)
	if err != nil {
		return err
	}

	volume, found, err := buildArtifact.Volume(step.build.TeamID())
	if err != nil {
		return err
	}

	if !found {
		return ArtifactVolumeNotFoundErr(buildArtifact.Name())
	}

	workerVolume, found, err := step.workerClient.LookupVolume(logger, volume.Handle())
	if err != nil {
		return err
	}

	if !found {
		return ArtifactVolumeNotFoundErr(buildArtifact.Name())
	}

	logger.Info("register-artifact-source", lager.Data{
		"handle": workerVolume.Handle(),
	})

	source := NewTaskArtifactSource(workerVolume)
	state.Artifacts().RegisterSource(artifact.Name(step.plan.UserArtifact.Name), source)

	step.succeeded = true

	return nil
}

func (step *ArtifactStep) Succeeded() bool {
	return step.succeeded
}
