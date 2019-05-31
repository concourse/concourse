package exec

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/db"
	"github.com/concourse/concourse/v5/atc/exec/artifact"
	"github.com/concourse/concourse/v5/atc/worker"
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

	source, found := state.Artifacts().SourceFor(artifact.Name(outputName))
	if !found {
		return ArtifactNotFoundError{outputName}
	}

	volume, ok := source.(worker.Volume)
	if !ok {
		return ArtifactNotFoundError{outputName}
	}

	artifact, err := volume.InitializeArtifact(outputName, step.build.ID())
	if err != nil {
		return err
	}

	logger.Info("initialize-artifact-from-source", lager.Data{
		"handle":      volume.Handle(),
		"artifact_id": artifact.ID(),
	})

	step.succeeded = true

	return nil
}

func (step *ArtifactOutputStep) Succeeded() bool {
	return step.succeeded
}
