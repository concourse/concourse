package exec

import (
	"context"
	"fmt"
	"io"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
)

//go:generate counterfeiter . PutDelegate

type PutDelegate interface {
	ImageVersionDetermined(db.UsedResourceCache) error

	Stdout() io.Writer
	Stderr() io.Writer

	Variables() vars.CredVarsTracker

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, ExitStatus, runtime.VersionResult)
	Errored(lager.Logger, string)

	SaveOutput(lager.Logger, atc.PutPlan, atc.Source, atc.VersionedResourceTypes, runtime.VersionResult)
}

// PutStep produces a resource version using preconfigured params and any data
// available in the worker.ArtifactRepository.
type PutStep struct {
	planID                atc.PlanID
	plan                  atc.PutPlan
	metadata              StepMetadata
	containerMetadata     db.ContainerMetadata
	resourceFactory       resource.ResourceFactory
	resourceConfigFactory db.ResourceConfigFactory
	strategy              worker.ContainerPlacementStrategy
	workerClient          worker.Client
	delegate              PutDelegate
	succeeded             bool
}

func NewPutStep(
	planID atc.PlanID,
	plan atc.PutPlan,
	metadata StepMetadata,
	containerMetadata db.ContainerMetadata,
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	strategy worker.ContainerPlacementStrategy,
	workerClient worker.Client,
	delegate PutDelegate,
) *PutStep {
	return &PutStep{
		planID:                planID,
		plan:                  plan,
		metadata:              metadata,
		containerMetadata:     containerMetadata,
		resourceFactory:       resourceFactory,
		resourceConfigFactory: resourceConfigFactory,
		workerClient:          workerClient,
		strategy:              strategy,
		delegate:              delegate,
	}
}

// Run chooses a worker that supports the step's resource type and creates a
// container.
//
// All worker.ArtifactSources present in the worker.ArtifactRepository are then brought into
// the container, using volumes if possible, and streaming content over if not.
//
// The resource's put script is then invoked. If the context is canceled, the
// script will be interrupted.
func (step *PutStep) Run(ctx context.Context, state RunState) error {
	ctx, span := tracing.StartSpan(ctx, "put", tracing.Attrs{
		"team":     step.metadata.TeamName,
		"pipeline": step.metadata.PipelineName,
		"job":      step.metadata.JobName,
		"build":    step.metadata.BuildName,
		"resource": step.plan.Resource,
		"name":     step.plan.Name,
	})

	err := step.run(ctx, state)
	tracing.End(span, err)

	return err
}

func (step *PutStep) run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("put-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	step.delegate.Initializing(logger)

	variables := step.delegate.Variables()

	source, err := creds.NewSource(variables, step.plan.Source).Evaluate()
	if err != nil {
		return err
	}

	params, err := creds.NewParams(variables, step.plan.Params).Evaluate()
	if err != nil {
		return err
	}

	resourceTypes, err := creds.NewVersionedResourceTypes(variables, step.plan.VersionedResourceTypes).Evaluate()
	if err != nil {
		return err
	}

	var putInputs PutInputs
	if step.plan.Inputs == nil {
		// Put step defaults to all inputs if not specified
		putInputs = NewAllInputs()
	} else if step.plan.Inputs.All {
		putInputs = NewAllInputs()
	} else {
		// Covers both cases where inputs are specified and when there are no
		// inputs specified and "all" field is given a false boolean, which will
		// result in no inputs attached
		putInputs = NewSpecificInputs(step.plan.Inputs.Specified)
	}

	containerInputs, err := putInputs.FindAll(state.ArtifactRepository())
	if err != nil {
		return err
	}

	env := step.metadata.Env()
	env = append(env, fmt.Sprintf("RESOURCE_NAME=%s", step.plan.Name))

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: step.plan.Type,
		},
		Tags:           step.plan.Tags,
		TeamID:         step.metadata.TeamID,
		Dir:            step.containerMetadata.WorkingDirectory,
		Env:            env,
		ArtifactByPath: containerInputs,
	}

	workerSpec := worker.WorkerSpec{
		ResourceType:  step.plan.Type,
		Tags:          step.plan.Tags,
		TeamID:        step.metadata.TeamID,
		ResourceTypes: resourceTypes,
	}

	owner := db.NewBuildStepContainerOwner(step.metadata.BuildID, step.planID, step.metadata.TeamID)

	containerSpec.BindMounts = []worker.BindMountSource{
		&worker.CertsVolumeMount{Logger: logger},
	}

	imageSpec := worker.ImageFetcherSpec{
		ResourceTypes: resourceTypes,
		Delegate:      step.delegate,
	}

	processSpec := runtime.ProcessSpec{
		Path:         "/opt/resource/out",
		Args:         []string{resource.ResourcesDir("put")},
		StdoutWriter: step.delegate.Stdout(),
		StderrWriter: step.delegate.Stderr(),
	}

	resourceToPut := step.resourceFactory.NewResource(source, params, nil)

	result := step.workerClient.RunPutStep(
		ctx,
		logger,
		owner,
		containerSpec,
		workerSpec,
		step.strategy,
		step.containerMetadata,
		imageSpec,
		processSpec,
		step.delegate,
		resourceToPut,
	)

	versionResult := result.VersionResult
	err = result.Err

	if err != nil {
		logger.Error("failed-to-put-resource", err)

		if err, ok := err.(runtime.ErrResourceScriptFailed); ok {
			step.delegate.Finished(logger, ExitStatus(err.ExitStatus), runtime.VersionResult{})
			return nil
		}

		return err
	}

	// step.plan.Resource maps to an actual resource that may have been used outside of a pipeline context.
	// Hence, if it was used outside the pipeline context, we don't want to save the output.
	if step.plan.Resource != "" {
		step.delegate.SaveOutput(logger, step.plan, source, resourceTypes, versionResult)
	}

	state.StoreResult(step.planID, versionResult)

	step.succeeded = true

	step.delegate.Finished(logger, 0, versionResult)

	return nil

}

// Succeeded returns true if the resource script exited successfully.
func (step *PutStep) Succeeded() bool {
	return step.succeeded
}
