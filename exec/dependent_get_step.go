package exec

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

type dependentGetStep struct {
	logger         lager.Logger
	sourceName     SourceName
	workerPool     worker.Client
	resourceConfig atc.ResourceConfig
	params         atc.Params
	stepMetadata   StepMetadata
	session        resource.Session
	tags           atc.Tags
	delegate       ResourceDelegate
	trackerFactory TrackerFactory
}

func newDependentGetStep(
	logger lager.Logger,
	sourceName SourceName,
	workerPool worker.Client,
	resourceConfig atc.ResourceConfig,
	params atc.Params,
	stepMetadata StepMetadata,
	session resource.Session,
	tags atc.Tags,
	delegate ResourceDelegate,
	trackerFactory TrackerFactory,
) dependentGetStep {
	return dependentGetStep{
		logger:         logger,
		sourceName:     sourceName,
		workerPool:     workerPool,
		resourceConfig: resourceConfig,
		params:         params,
		stepMetadata:   stepMetadata,
		session:        session,
		tags:           tags,
		delegate:       delegate,
		trackerFactory: trackerFactory,
	}
}

func (step dependentGetStep) Using(prev Step, repo *SourceRepository) Step {
	var info VersionInfo
	prev.Result(&info)

	return newGetStep(
		step.logger,
		step.sourceName,
		step.workerPool,
		step.resourceConfig,
		info.Version,
		step.params,
		step.stepMetadata,
		step.session,
		step.tags,
		step.delegate,
		step.trackerFactory,
	).Using(prev, repo)
}
