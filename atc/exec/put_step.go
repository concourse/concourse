package exec

import (
	"context"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/creds"
	"github.com/concourse/concourse/v5/atc/db"
	"github.com/concourse/concourse/v5/atc/resource"
	"github.com/concourse/concourse/v5/atc/worker"
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
	build db.Build

	name         string
	resourceType string
	resource     string
	source       creds.Source
	params       creds.Params
	tags         atc.Tags
	inputs       PutInputs

	delegate              PutDelegate
	pool                  worker.Pool
	resourceConfigFactory db.ResourceConfigFactory
	planID                atc.PlanID
	containerMetadata     db.ContainerMetadata
	stepMetadata          StepMetadata

	resourceTypes creds.VersionedResourceTypes

	versionInfo VersionInfo
	succeeded   bool

	strategy        worker.ContainerPlacementStrategy
	resourceFactory resource.ResourceFactory
}

func NewPutStep(
	build db.Build,
	name string,
	resourceType string,
	resourceName string,
	source creds.Source,
	params creds.Params,
	tags atc.Tags,
	inputs PutInputs,
	delegate PutDelegate,
	pool worker.Pool,
	resourceConfigFactory db.ResourceConfigFactory,
	planID atc.PlanID,
	containerMetadata db.ContainerMetadata,
	stepMetadata StepMetadata,
	resourceTypes creds.VersionedResourceTypes,
	strategy worker.ContainerPlacementStrategy,
	resourceFactory resource.ResourceFactory,
) *PutStep {
	return &PutStep{
		build: build,

		resourceType:          resourceType,
		name:                  name,
		resource:              resourceName,
		source:                source,
		params:                params,
		tags:                  tags,
		inputs:                inputs,
		delegate:              delegate,
		pool:                  pool,
		resourceConfigFactory: resourceConfigFactory,
		planID:                planID,
		containerMetadata:     containerMetadata,
		stepMetadata:          stepMetadata,
		resourceTypes:         resourceTypes,
		strategy:              strategy,
		resourceFactory:       resourceFactory,
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
		"step-name": step.name,
		"job-id":    step.build.JobID(),
	})

	step.delegate.Initializing(logger)

	containerInputs, err := step.inputs.FindAll(state.Artifacts())
	if err != nil {
		return err
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: step.resourceType,
		},
		Tags:   step.tags,
		TeamID: step.build.TeamID(),

		Dir: resource.ResourcesDir("put"),

		Env: step.stepMetadata.Env(),

		Inputs: containerInputs,
	}

	workerSpec := worker.WorkerSpec{
		ResourceType:  step.resourceType,
		Tags:          step.tags,
		TeamID:        step.build.TeamID(),
		ResourceTypes: step.resourceTypes,
	}

	owner := db.NewBuildStepContainerOwner(step.build.ID(), step.planID, step.build.TeamID())
	chosenWorker, err := step.pool.FindOrChooseWorkerForContainer(logger, owner, containerSpec, workerSpec, step.strategy)
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
		step.containerMetadata,
		containerSpec,
		step.resourceTypes,
	)
	if err != nil {
		return err
	}

	source, err := step.source.Evaluate()
	if err != nil {
		return err
	}

	params, err := step.params.Evaluate()
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

	step.versionInfo = VersionInfo{
		Version:  versionedSource.Version(),
		Metadata: versionedSource.Metadata(),
	}

	if step.resource != "" {
		logger = logger.WithData(lager.Data{"step": step.name, "resource": step.resource, "resource-type": step.resourceType, "version": step.versionInfo.Version})
		err = step.build.SaveOutput(logger, step.resourceType, source, step.resourceTypes, step.versionInfo.Version, db.NewResourceConfigMetadataFields(step.versionInfo.Metadata), step.name, step.resource)
		if err != nil {
			logger.Error("failed-to-save-output", err)
			return err
		}
	}

	state.StoreResult(step.planID, step.versionInfo)

	step.succeeded = true

	step.delegate.Finished(logger, 0, step.versionInfo)

	return nil
}

// VersionInfo returns the info of the pushed version.
func (step *PutStep) VersionInfo() VersionInfo {
	return step.versionInfo
}

// Succeeded returns true if the resource script exited successfully.
func (step *PutStep) Succeeded() bool {
	return step.succeeded
}
