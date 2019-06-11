package exec

import (
	"context"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

//go:generate counterfeiter . PutDelegate

type PutDelegate interface {
	BuildStepDelegate

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, ExitStatus, VersionInfo)
}

// PutStep produces a resource version using preconfigured params and any data
// available in the worker.ArtifactRepository.
type PutStep struct {
	planID                atc.PlanID
	plan                  atc.PutPlan
	build                 db.Build
	stepMetadata          StepMetadata
	containerMetadata     db.ContainerMetadata
	secrets               creds.Secrets
	resourceFactory       resource.ResourceFactory
	resourceConfigFactory db.ResourceConfigFactory
	strategy              worker.ContainerPlacementStrategy
	pool                  worker.Pool
	delegate              PutDelegate
	succeeded             bool
}

func NewPutStep(
	planID atc.PlanID,
	plan atc.PutPlan,
	build db.Build,
	stepMetadata StepMetadata,
	containerMetadata db.ContainerMetadata,
	secrets creds.Secrets,
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	strategy worker.ContainerPlacementStrategy,
	pool worker.Pool,
	delegate PutDelegate,
) *PutStep {
	return &PutStep{
		planID:                planID,
		plan:                  plan,
		build:                 build,
		stepMetadata:          stepMetadata,
		containerMetadata:     containerMetadata,
		secrets:               secrets,
		resourceFactory:       resourceFactory,
		resourceConfigFactory: resourceConfigFactory,
		pool:                  pool,
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
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("put-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.build.JobID(),
	})

	step.delegate.Initializing(logger)

	variables := creds.NewVariables(step.secrets, step.build.TeamName(), step.build.PipelineName())

	source, err := creds.NewSource(variables, step.plan.Source).Evaluate()
	if err != nil {
		return err
	}

	params, err := creds.NewParams(variables, step.plan.Params).Evaluate()
	if err != nil {
		return err
	}

	resourceTypes := creds.NewVersionedResourceTypes(variables, step.plan.VersionedResourceTypes)

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

	containerInputs, err := putInputs.FindAll(state.Artifacts())
	if err != nil {
		return err
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: step.plan.Type,
		},
		Tags:   step.plan.Tags,
		TeamID: step.build.TeamID(),

		Dir: resource.ResourcesDir("put"),

		Env: step.stepMetadata.Env(),

		Inputs: containerInputs,
	}

	workerSpec := worker.WorkerSpec{
		ResourceType:  step.plan.Type,
		Tags:          step.plan.Tags,
		TeamID:        step.build.TeamID(),
		ResourceTypes: resourceTypes,
	}

	owner := db.NewBuildStepContainerOwner(step.build.ID(), step.planID, step.build.TeamID())

	chosenWorker, err := step.pool.FindOrChooseWorkerForContainer(
		ctx,
		logger,
		owner,
		containerSpec,
		step.containerMetadata,
		workerSpec,
		step.strategy,
	)
	if err != nil {
		return err
	}

	containerSpec.BindMounts = []worker.BindMountSource{
		&worker.CertsVolumeMount{Logger: logger},
	}

	container, err := chosenWorker.FindOrCreateContainer(
		ctx,
		logger,
		step.delegate,
		owner,
		containerSpec,
		resourceTypes,
	)
	if err != nil {
		return err
	}

	step.delegate.Starting(logger)

	putResource := step.resourceFactory.NewResourceForContainer(container)
	versionedSource, err := putResource.Put(
		ctx,
		resource.IOConfig{
			Stdout: step.delegate.Stdout(),
			Stderr: step.delegate.Stderr(),
		},
		source,
		params,
	)

	if err != nil {
		logger.Error("failed-to-put-resource", err)

		if err, ok := err.(resource.ErrResourceScriptFailed); ok {
			step.delegate.Finished(logger, ExitStatus(err.ExitStatus), VersionInfo{})
			return nil
		}

		return err
	}

	versionInfo := VersionInfo{
		Version:  versionedSource.Version(),
		Metadata: versionedSource.Metadata(),
	}

	if step.plan.Resource != "" {
		logger = logger.WithData(lager.Data{"step": step.plan.Name, "resource": step.plan.Resource, "resource-type": step.plan.Type, "version": versionInfo.Version})
		err = step.build.SaveOutput(logger, step.plan.Type, source, resourceTypes, versionInfo.Version, db.NewResourceConfigMetadataFields(versionInfo.Metadata), step.plan.Name, step.plan.Resource)
		if err != nil {
			logger.Error("failed-to-save-output", err)
			return err
		}
	}

	state.StoreResult(step.planID, versionInfo)

	step.succeeded = true

	step.delegate.Finished(logger, 0, versionInfo)

	return nil
}

// Succeeded returns true if the resource script exited successfully.
func (step *PutStep) Succeeded() bool {
	return step.succeeded
}
