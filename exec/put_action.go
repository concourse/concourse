package exec

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

// PutAction produces a resource version using preconfigured params and any data
// available in the worker.ArtifactRepository.
type PutAction struct {
	Type     string
	Name     string
	Resource string
	Source   creds.Source
	Params   atc.Params
	Tags     atc.Tags

	imageFetchingDelegate ImageFetchingDelegate
	resourceFactory       resource.ResourceFactory
	teamID                int
	buildID               int
	planID                atc.PlanID
	containerMetadata     db.ContainerMetadata
	stepMetadata          StepMetadata

	resourceTypes creds.VersionedResourceTypes

	versionInfo VersionInfo
	exitStatus  ExitStatus
}

func NewPutAction(
	resourceType string,
	name string,
	resourceName string,
	source creds.Source,
	params atc.Params,
	tags atc.Tags,
	imageFetchingDelegate ImageFetchingDelegate,
	resourceFactory resource.ResourceFactory,
	teamID int,
	buildID int,
	planID atc.PlanID,
	containerMetadata db.ContainerMetadata,
	stepMetadata StepMetadata,
	resourceTypes creds.VersionedResourceTypes,
) *PutAction {
	return &PutAction{
		Type:     resourceType,
		Name:     name,
		Resource: resourceName,
		Source:   source,
		Params:   params,
		Tags:     tags,
		imageFetchingDelegate: imageFetchingDelegate,
		resourceFactory:       resourceFactory,
		teamID:                teamID,
		buildID:               buildID,
		planID:                planID,
		containerMetadata:     containerMetadata,
		stepMetadata:          stepMetadata,
		resourceTypes:         resourceTypes,
	}
}

// Run chooses a worker that supports the step's resource type and creates a
// container.
//
// All worker.ArtifactSources present in the worker.ArtifactRepository are then brought into
// the container, using volumes if possible, and streaming content over if not.
//
// The resource's put script is then invoked. The PutStep is ready as soon as
// the resource's script starts, and signals will be forwarded to the script.
func (action *PutAction) Run(
	logger lager.Logger,
	repository *worker.ArtifactRepository,

	// TODO: consider passing these as context
	signals <-chan os.Signal,
	ready chan<- struct{},
) error {
	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: action.Type,
		},
		Tags:   action.Tags,
		TeamID: action.teamID,

		Dir: resource.ResourcesDir("put"),

		Env: action.stepMetadata.Env(),
	}

	for name, source := range repository.AsMap() {
		containerSpec.Inputs = append(containerSpec.Inputs, &putInputSource{
			name:   name,
			source: PutResourceSource{source},
		})
	}

	putResource, err := action.resourceFactory.NewResource(
		logger,
		signals,
		db.NewBuildStepContainerOwner(action.buildID, action.planID),
		action.containerMetadata,
		containerSpec,
		action.resourceTypes,
		action.imageFetchingDelegate,
	)
	if err != nil {
		return err
	}

	source, err := action.Source.Evaluate()
	if err != nil {
		return err
	}

	versionedSource, err := putResource.Put(
		resource.IOConfig{
			Stdout: action.imageFetchingDelegate.Stdout(),
			Stderr: action.imageFetchingDelegate.Stderr(),
		},
		source,
		action.Params,
		signals,
		ready,
	)

	if err != nil {
		if err, ok := err.(resource.ErrResourceScriptFailed); ok {
			action.exitStatus = ExitStatus(err.ExitStatus)
			return nil
		}
		return err
	}

	action.versionInfo = VersionInfo{
		Version:  versionedSource.Version(),
		Metadata: versionedSource.Metadata(),
	}
	action.exitStatus = ExitStatus(0)

	return nil
}

// VersionInfo returns the produced resource's version
// and metadata.
func (action *PutAction) VersionInfo() VersionInfo {
	return action.versionInfo
}

// ExitStatus returns exit status of resource put script.
func (action *PutAction) ExitStatus() ExitStatus {
	return action.exitStatus
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
