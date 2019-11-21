package exec

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/worker"
)

type ArtifactNotFoundError struct {
	ArtifactName string
}

func (e ArtifactNotFoundError) Error() string {
	return fmt.Sprintf("artifact '%s' not found", e.ArtifactName)
}

type ArtifactOutputStep struct {
	plan         atc.Plan
	build        db.Build
	workerClient worker.Client
	delegate     BuildStepDelegate
	succeeded    bool
}

func NewArtifactOutputStep(plan atc.Plan, build db.Build, workerClient worker.Client, delegate BuildStepDelegate) Step {
	return &ArtifactOutputStep{
		plan:         plan,
		build:        build,
		workerClient: workerClient,
		delegate:     delegate,
	}
}

func (step *ArtifactOutputStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx).WithData(lager.Data{
		"plan-id": step.plan.ID,
	})

	outputName := step.plan.ArtifactOutput.Name

	buildArtifact, found := state.ArtifactRepository().ArtifactFor(build.ArtifactName(outputName))
	if !found {
		return ArtifactNotFoundError{outputName}
	}

	// TODO Step shouldn't know about volumes
	volume, found, err := step.workerClient.FindVolume(logger, step.build.TeamID(), buildArtifact.ID())
	if err != nil {
		return err
	}

	if !found {
		return ArtifactNotFoundError{outputName}
	}

	dbWorkerArtifact, err := volume.InitializeArtifact(outputName, step.build.ID())
	if err != nil {
		return err
	}

	logger.Info("initialize-artifact-from-source", lager.Data{
		"handle":      volume.Handle(),
		"artifact_id": dbWorkerArtifact.ID(),
	})

	step.succeeded = true

	return nil
}

func (step *ArtifactOutputStep) Succeeded() bool {
	return step.succeeded
}
