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
	"go.opentelemetry.io/otel/trace"
)

//counterfeiter:generate . PutDelegateFactory
type PutDelegateFactory interface {
	PutDelegate(state RunState) PutDelegate
}

//counterfeiter:generate . PutDelegate
type PutDelegate interface {
	StartSpan(context.Context, string, tracing.Attrs) (context.Context, trace.Span)

	FetchImage(context.Context, atc.Plan, *atc.Plan, bool) (runtime.ImageSpec, db.ResourceCache, error)

	Stdout() io.Writer
	Stderr() io.Writer

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, ExitStatus, resource.VersionResult)
	Errored(lager.Logger, string)

	WaitingForWorker(lager.Logger)
	SelectedWorker(lager.Logger, string)

	SaveOutput(lager.Logger, atc.PutPlan, atc.Source, db.ResourceCache, resource.VersionResult)
}

// PutStep produces a resource version using preconfigured params and any data
// available in the worker.ArtifactRepository.
type PutStep struct {
	planID            atc.PlanID
	plan              atc.PutPlan
	metadata          StepMetadata
	containerMetadata db.ContainerMetadata
	strategy          worker.PlacementStrategy
	workerPool        Pool
	delegateFactory   PutDelegateFactory
}

func NewPutStep(
	planID atc.PlanID,
	plan atc.PutPlan,
	metadata StepMetadata,
	containerMetadata db.ContainerMetadata,
	strategy worker.PlacementStrategy,
	workerPool Pool,
	delegateFactory PutDelegateFactory,
) Step {
	return &PutStep{
		planID:            planID,
		plan:              plan,
		metadata:          metadata,
		containerMetadata: containerMetadata,
		workerPool:        workerPool,
		strategy:          strategy,
		delegateFactory:   delegateFactory,
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

	containerInputs, err := putInputs.FindAll(state.ArtifactRepository())
	if err != nil {
		return false, err
	}

	workerSpec := worker.Spec{
		Tags:   step.plan.Tags,
		TeamID: step.metadata.TeamID,

		// Used to filter out non-Linux workers, simply because they don't support
		// base resource types
		ResourceType: step.plan.TypeImage.BaseType,
	}

	var imageSpec runtime.ImageSpec
	var imageResourceCache db.ResourceCache
	if step.plan.TypeImage.GetPlan != nil {
		imageSpec, imageResourceCache, err = delegate.FetchImage(ctx, *step.plan.TypeImage.GetPlan, step.plan.TypeImage.CheckPlan, step.plan.TypeImage.Privileged)
		if err != nil {
			return false, err
		}
	} else {
		imageSpec.ResourceType = step.plan.TypeImage.BaseType
	}

	containerSpec := runtime.ContainerSpec{
		TeamID:   step.metadata.TeamID,
		TeamName: step.metadata.TeamName,
		JobID:    step.metadata.JobID,

		ImageSpec: imageSpec,

		Env:  step.metadata.Env(),
		Type: db.ContainerTypePut,

		Dir: step.containerMetadata.WorkingDirectory,

		Inputs: containerInputs,

		CertsBindMount: true,
	}
	tracing.Inject(ctx, &containerSpec)

	owner := db.NewBuildStepContainerOwner(step.metadata.BuildID, step.planID, step.metadata.TeamID)

	worker, err := step.workerPool.FindOrSelectWorker(ctx, owner, containerSpec, workerSpec, step.strategy, delegate)
	if err != nil {
		return false, err
	}

	delegate.SelectedWorker(logger, worker.Name())

	defer func() {
		step.workerPool.ReleaseWorker(
			logger,
			containerSpec,
			worker,
			step.strategy,
		)
	}()

	ctx, cancel, err := MaybeTimeout(ctx, step.plan.Timeout)
	if err != nil {
		return false, err
	}
	ctx = lagerctx.NewContext(ctx, logger)

	defer cancel()

	container, _, err := worker.FindOrCreateContainer(ctx, owner, step.containerMetadata, containerSpec)
	if err != nil {
		return false, err
	}

	delegate.Starting(logger)
	versionResult, processResult, err := resource.Resource{
		Source: source,
		Params: params,
	}.Put(ctx, container, delegate.Stderr())
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			delegate.Errored(logger, TimeoutLogMessage)
			return false, nil
		}

		return false, err
	}

	if processResult.ExitStatus != 0 {
		delegate.Finished(logger, ExitStatus(processResult.ExitStatus), resource.VersionResult{})
		return false, nil
	}

	// step.plan.Resource maps to an actual resource that may have been used outside of a pipeline context.
	// Hence, if it was used outside the pipeline context, we don't want to save the output.
	if step.plan.Resource != "" {
		delegate.SaveOutput(logger, step.plan, source, imageResourceCache, versionResult)
	}

	state.StoreResult(step.planID, versionResult.Version)

	delegate.Finished(logger, 0, versionResult)

	return true, nil
}
