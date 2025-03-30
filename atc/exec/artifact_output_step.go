package exec

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/runtime"
)

type ArtifactNotFoundError struct {
	ArtifactName string
}

func (e ArtifactNotFoundError) Error() string {
	return fmt.Sprintf("artifact '%s' not found", e.ArtifactName)
}

type ArtifactNotVolumeError struct {
	ArtifactName string
}

func (e ArtifactNotVolumeError) Error() string {
	return fmt.Sprintf("artifact '%s' is not a volume", e.ArtifactName)
}

type ArtifactOutputStep struct {
	plan       atc.Plan
	build      db.Build
	workerPool Pool
}

func NewArtifactOutputStep(plan atc.Plan, build db.Build, workerPool Pool) Step {
	return &ArtifactOutputStep{
		plan:       plan,
		build:      build,
		workerPool: workerPool,
	}
}

func (step *ArtifactOutputStep) Run(ctx context.Context, state RunState) (bool, error) {
	logger := lagerctx.FromContext(ctx).WithData(lager.Data{
		"plan-id": step.plan.ID,
	})

	outputName := step.plan.ArtifactOutput.Name

	artifact, _, found := state.ArtifactRepository().ArtifactFor(build.ArtifactName(outputName))
	if !found {
		return false, ArtifactNotFoundError{outputName}
	}

	volume, ok := artifact.(runtime.Volume)
	if !ok {
		return false, ArtifactNotVolumeError{outputName}
	}

	dbWorkerArtifact, err := volume.DBVolume().InitializeArtifact(outputName, step.build.ID())
	if err != nil {
		return false, err
	}

	logger.Debug("initialize-artifact-from-source", lager.Data{
		"handle":      volume.Handle(),
		"artifact_id": dbWorkerArtifact.ID(),
	})

	return true, nil
}
