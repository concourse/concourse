package exec

import (
	"context"
	"errors"
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
	"go.opentelemetry.io/otel/api/trace"
)

//go:generate counterfeiter . PutDelegateFactory

type PutDelegateFactory interface {
	PutDelegate(state RunState) PutDelegate
}

//go:generate counterfeiter . PutDelegate

type PutDelegate interface {
	StartSpan(context.Context, string, tracing.Attrs) (context.Context, trace.Span)

	FetchImage(context.Context, atc.ImageResource, atc.VersionedResourceTypes, bool) (worker.ImageSpec, error)

	Stdout() io.Writer
	Stderr() io.Writer

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, ExitStatus, runtime.VersionResult)
	SelectedWorker(lager.Logger, string)
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
	workerPool            worker.Pool
	artifactSourcer       worker.ArtifactSourcer
	delegateFactory       PutDelegateFactory
}

func NewPutStep(
	planID atc.PlanID,
	plan atc.PutPlan,
	metadata StepMetadata,
	containerMetadata db.ContainerMetadata,
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	strategy worker.ContainerPlacementStrategy,
	workerPool worker.Pool,
	artifactSourcer worker.ArtifactSourcer,
	delegateFactory PutDelegateFactory,
) Step {
	return &PutStep{
		planID:                planID,
		plan:                  plan,
		metadata:              metadata,
		containerMetadata:     containerMetadata,
		resourceFactory:       resourceFactory,
		resourceConfigFactory: resourceConfigFactory,
		workerPool:            workerPool,
		artifactSourcer:       artifactSourcer,
		strategy:              strategy,
		delegateFactory:       delegateFactory,
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
func (step *PutStep) Run(ctx context.Context, state RunState) (bool, error) {
	delegate := step.delegateFactory.PutDelegate(state)
	ctx, span := delegate.StartSpan(ctx, "put", tracing.Attrs{
		"name":     step.plan.Name,
		"resource": step.plan.Resource,
	})

	ok, err := step.run(ctx, state, delegate)
	tracing.End(span, err)

	return ok, err
}

func (step *PutStep) run(ctx context.Context, state RunState, delegate PutDelegate) (bool, error) {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("put-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	delegate.Initializing(logger)

	source, err := creds.NewSource(state, step.plan.Source).Evaluate()
	if err != nil {
		return false, err
	}

	params, err := creds.NewParams(state, step.plan.Params).Evaluate()
	if err != nil {
		return false, err
	}

	resourceTypes, err := creds.NewVersionedResourceTypes(state, step.plan.VersionedResourceTypes).Evaluate()
	if err != nil {
		return false, err
	}

	var putInputs PutInputs
	if step.plan.Inputs == nil {
		// Put step defaults to all inputs if not specified
		putInputs = NewAllInputs()
	} else if step.plan.Inputs.All {
		putInputs = NewAllInputs()
	} else if step.plan.Inputs.Detect {
		putInputs = NewDetectInputs(step.plan.Params)
	} else {
		// Covers both cases where inputs are specified and when there are no
		// inputs specified and "all" field is given a false boolean, which will
		// result in no inputs attached
		putInputs = NewSpecificInputs(step.plan.Inputs.Specified)
	}

	inputsMap, err := putInputs.FindAll(state.ArtifactRepository())
	if err != nil {
		return false, err
	}

	containerInputs, err := step.artifactSourcer.SourceInputsAndCaches(logger, step.metadata.TeamID, inputsMap)
	if err != nil {
		return false, err
	}

	workerSpec := worker.WorkerSpec{
		Tags:         step.plan.Tags,
		TeamID:       step.metadata.TeamID,
		ResourceType: step.plan.VersionedResourceTypes.Base(step.plan.Type),
	}

	var imageSpec worker.ImageSpec
	resourceType, found := step.plan.VersionedResourceTypes.Lookup(step.plan.Type)
	if found {
		image := atc.ImageResource{
			Name:    resourceType.Name,
			Type:    resourceType.Type,
			Source:  resourceType.Source,
			Params:  resourceType.Params,
			Version: resourceType.Version,
			Tags:    resourceType.Tags,
		}
		if len(image.Tags) == 0 {
			image.Tags = step.plan.Tags
		}

		types := step.plan.VersionedResourceTypes.Without(step.plan.Type)

		var err error
		imageSpec, err = delegate.FetchImage(ctx, image, types, resourceType.Privileged)
		if err != nil {
			return false, err
		}
	} else {
		imageSpec.ResourceType = step.plan.Type
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: imageSpec,
		TeamID:    step.metadata.TeamID,

		Dir: step.containerMetadata.WorkingDirectory,

		Env: step.metadata.Env(),

		Inputs: containerInputs,
	}
	tracing.Inject(ctx, &containerSpec)

	owner := db.NewBuildStepContainerOwner(step.metadata.BuildID, step.planID, step.metadata.TeamID)

	containerSpec.BindMounts = []worker.BindMountSource{
		&worker.CertsVolumeMount{Logger: logger},
	}

	processSpec := runtime.ProcessSpec{
		Path:         "/opt/resource/out",
		Args:         []string{resource.ResourcesDir("put")},
		StdoutWriter: delegate.Stdout(),
		StderrWriter: delegate.Stderr(),
	}

	resourceToPut := step.resourceFactory.NewResource(source, params, nil)

	processCtx, cancel, err := MaybeTimeout(ctx, step.plan.Timeout)
	if err != nil {
		return false, err
	}

	defer cancel()

	worker, _, err := step.workerPool.WaitForWorker(
		lagerctx.NewContext(processCtx, logger),
		owner,
		containerSpec,
		workerSpec,
		step.strategy,
	)
	if err != nil {
		return false, err
	}
	delegate.SelectedWorker(logger, worker.Name())

	result, err := worker.RunPutStep(
		lagerctx.NewContext(processCtx, logger),
		owner,
		containerSpec,
		step.containerMetadata,
		processSpec,
		delegate,
		resourceToPut,
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			delegate.Errored(logger, TimeoutLogMessage)
			return false, nil
		}

		return false, err
	}

	if result.ExitStatus != 0 {
		delegate.Finished(logger, ExitStatus(result.ExitStatus), runtime.VersionResult{})
		return false, nil
	}

	versionResult := result.VersionResult
	// step.plan.Resource maps to an actual resource that may have been used outside of a pipeline context.
	// Hence, if it was used outside the pipeline context, we don't want to save the output.
	if step.plan.Resource != "" {
		delegate.SaveOutput(logger, step.plan, source, resourceTypes, versionResult)
	}

	state.StoreResult(step.planID, versionResult)

	delegate.Finished(logger, 0, versionResult)

	return true, nil
}
