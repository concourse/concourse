package exec

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type PutAction struct {
	Type         string
	Name         string
	Resource     string
	Source       atc.Source
	Params       atc.Params
	Tags         atc.Tags
	RootFSSource RootFSSource

	// TODO: can we remove these dependencies?
	imageFetchingDelegate ImageFetchingDelegate
	resourceFactory       resource.ResourceFactory
	teamID                int
	buildID               int
	planID                atc.PlanID
	containerMetadata     db.ContainerMetadata
	stepMetadata          StepMetadata

	// TODO: remove after all actions are introduced
	resourceTypes atc.VersionedResourceTypes

	result *VersionInfo
}

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
			source: resourceSource{source},
		})
	}

	putResource, err := action.resourceFactory.NewResource(
		logger,
		signals,
		db.ForBuild(action.buildID),
		db.NewBuildStepContainerOwner(action.buildID, action.planID),
		action.containerMetadata,
		containerSpec,
		action.resourceTypes,
		action.imageFetchingDelegate,
	)
	if err != nil {
		return err
	}

	versionedSource, err := putResource.Put(
		resource.IOConfig{
			Stdout: action.imageFetchingDelegate.Stdout(),
			Stderr: action.imageFetchingDelegate.Stderr(),
		},
		action.Source,
		action.Params,
		signals,
		ready,
	)

	if err != nil {
		return err
	}

	action.result = &VersionInfo{
		Version:  versionedSource.Version(),
		Metadata: versionedSource.Metadata(),
	}

	return nil
}

func (action *PutAction) Result() (VersionInfo, bool) {
	if action.result != nil {
		return *action.result, true
	}

	return VersionInfo{}, false
}

type resourceSource struct {
	worker.ArtifactSource
}

func (source resourceSource) StreamTo(dest worker.ArtifactDestination) error {
	return source.ArtifactSource.StreamTo(worker.ArtifactDestination(dest))
}

type putInputSource struct {
	name   worker.ArtifactName
	source worker.ArtifactSource
}

func (s *putInputSource) Name() worker.ArtifactName     { return s.name }
func (s *putInputSource) Source() worker.ArtifactSource { return s.source }

func (s *putInputSource) DestinationPath() string {
	return resource.ResourcesDir("put/" + string(s.name))
}
