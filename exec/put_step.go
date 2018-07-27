package exec

import (
	"context"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

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
	source       creds.Source
	params       creds.Params
	tags         atc.Tags

	resource string

	delegate          PutDelegate
	resourceFactory   resource.ResourceFactory
	planID            atc.PlanID
	containerMetadata db.ContainerMetadata
	stepMetadata      StepMetadata

	resourceTypes creds.VersionedResourceTypes

	versionInfo VersionInfo
	succeeded   bool
}

func NewPutStep(
	build db.Build,
	name string,
	resourceType string,
	resourceName string,
	source creds.Source,
	params creds.Params,
	tags atc.Tags,
	delegate PutDelegate,
	resourceFactory resource.ResourceFactory,
	planID atc.PlanID,
	containerMetadata db.ContainerMetadata,
	stepMetadata StepMetadata,
	resourceTypes creds.VersionedResourceTypes,
) *PutStep {
	return &PutStep{
		build: build,

		resourceType:      resourceType,
		name:              name,
		resource:          resourceName,
		source:            source,
		params:            params,
		tags:              tags,
		delegate:          delegate,
		resourceFactory:   resourceFactory,
		planID:            planID,
		containerMetadata: containerMetadata,
		stepMetadata:      stepMetadata,
		resourceTypes:     resourceTypes,
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

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: step.resourceType,
		},
		Tags:   step.tags,
		TeamID: step.build.TeamID(),

		Dir: resource.ResourcesDir("put"),

		Env: step.stepMetadata.Env(),
	}

	for name, source := range state.Artifacts().AsMap() {
		containerSpec.Inputs = append(containerSpec.Inputs, &putInputSource{
			name:   name,
			source: PutResourceSource{source},
		})
	}

	putResource, err := step.resourceFactory.NewResource(
		ctx,
		logger,
		db.NewBuildStepContainerOwner(step.build.ID(), step.planID),
		step.containerMetadata,
		containerSpec,
		step.resourceTypes,
		step.delegate,
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
		err = step.build.SaveOutput(
			db.VersionedResource{
				Resource: step.resource,
				Type:     step.resourceType,
				Version:  db.ResourceVersion(step.versionInfo.Version),
				Metadata: db.NewResourceMetadataFields(step.versionInfo.Metadata),
			},
		)
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

type PutResourceSource struct {
	worker.ArtifactSource
}

func (source PutResourceSource) StreamTo(dest worker.ArtifactDestination) error {
	return source.ArtifactSource.StreamTo(worker.ArtifactDestination(dest))
}

type putInputSource struct {
	name   worker.ArtifactName
	source worker.ArtifactSource
}

func (s *putInputSource) Source() worker.ArtifactSource { return s.source }

func (s *putInputSource) DestinationPath() string {
	return resource.ResourcesDir("put/" + string(s.name))
}
