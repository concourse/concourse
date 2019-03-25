package exec

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

type VersionNotFoundError struct {
	Space atc.Space
}

func (e *VersionNotFoundError) Error() string {
	return fmt.Sprintf("latest version not found within space %s", e.Space)
}

//go:generate counterfeiter . PutDelegate

type PutDelegate interface {
	BuildStepDelegate

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
	delegate     PutDelegate

	resourceFactory   resource.ResourceFactory
	pool              worker.Pool
	planID            atc.PlanID
	containerMetadata db.ContainerMetadata
	stepMetadata      StepMetadata

	resourceTypes creds.VersionedResourceTypes

	versionInfo VersionInfo
	succeeded   bool

	strategy worker.ContainerPlacementStrategy
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
	resourceFactory resource.ResourceFactory,
	pool worker.Pool,
	planID atc.PlanID,
	containerMetadata db.ContainerMetadata,
	stepMetadata StepMetadata,
	resourceTypes creds.VersionedResourceTypes,
	strategy worker.ContainerPlacementStrategy,
) *PutStep {
	return &PutStep{
		build: build,

		resourceType:      resourceType,
		name:              name,
		resource:          resourceName,
		source:            source,
		params:            params,
		tags:              tags,
		inputs:            inputs,
		delegate:          delegate,
		pool:              pool,
		resourceFactory:   resourceFactory,
		planID:            planID,
		containerMetadata: containerMetadata,
		stepMetadata:      stepMetadata,
		resourceTypes:     resourceTypes,
		strategy:          strategy,
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

		Dir: atc.ResourcesDir("put"),

		Env: step.stepMetadata.Env(),

		Inputs: containerInputs,
	}

	workerSpec := worker.WorkerSpec{
		ResourceType:  step.resourceType,
		Tags:          step.tags,
		TeamID:        step.build.TeamID(),
		ResourceTypes: step.resourceTypes,
	}

	containerSpec.BindMounts = []worker.BindMountSource{
		&worker.CertsVolumeMount{Logger: logger},
	}

	owner := db.NewBuildStepContainerOwner(step.build.ID(), step.planID, step.build.TeamID())

	chosenWorker, err := step.pool.FindOrChooseWorkerForContainer(logger, owner, containerSpec, workerSpec, step.strategy)
	if err != nil {
		return err
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

	putResource, err := step.resourceFactory.NewResourceForContainer(ctx, container)
	if err != nil {
		return err
	}

	versions, err := putResource.Put(
		ctx,
		NewPutEventHandler(),
		atc.IOConfig{
			Stdout: step.delegate.Stdout(),
			Stderr: step.delegate.Stderr(),
		},
		source,
		params,
	)
	if err != nil {
		logger.Error("failed-to-put-resource", err)

		if err, ok := err.(atc.ErrResourceScriptFailed); ok {
			step.delegate.Finished(logger, ExitStatus(err.ExitStatus), VersionInfo{})
			return nil
		}

		return err
	}

	if len(versions) != 0 {
		version := versions[len(versions)-1]

		step.versionInfo = VersionInfo{
			Version:  version.Version,
			Space:    version.Space,
			Metadata: version.Metadata,
		}

		if step.resource != "" {
			logger = logger.WithData(lager.Data{"step": step.name, "resource": step.resource, "resource-type": step.resourceType, "version": step.versionInfo.Version})

			pipeline, found, err := step.build.Pipeline()
			if err != nil {
				return err
			}

			if !found {
				return ErrPipelineNotFound{step.build.PipelineName()}
			}

			dbResource, found, err := pipeline.Resource(step.resource)
			if err != nil {
				return err
			}

			if !found {
				return ErrResourceNotFound{step.resource}
			}

			for _, v := range versions {
				err = step.build.SaveOutput(logger, v, step.name, step.resource)
				if err != nil {
					logger.Error("failed-to-save-output", err, lager.Data{"version": v.Version})
					return err
				}

				err = dbResource.SaveMetadata(v.Space, v.Version, v.Metadata)
				if err != nil {
					logger.Error("failed-to-save-metadata", err, lager.Data{"version": v.Version})
					return err
				}
			}
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
