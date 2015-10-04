package exec

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/pivotal-golang/lager"
)

type dependentGetStep struct {
	logger         lager.Logger
	sourceName     SourceName
	resourceConfig atc.ResourceConfig
	params         atc.Params
	stepMetadata   StepMetadata
	session        resource.Session
	tags           atc.Tags
	delegate       ResourceDelegate
	tracker        resource.Tracker
}

func newDependentGetStep(
	logger lager.Logger,
	sourceName SourceName,
	resourceConfig atc.ResourceConfig,
	params atc.Params,
	stepMetadata StepMetadata,
	session resource.Session,
	tags atc.Tags,
	delegate ResourceDelegate,
	tracker resource.Tracker,
) dependentGetStep {
	return dependentGetStep{
		logger:         logger,
		sourceName:     sourceName,
		resourceConfig: resourceConfig,
		params:         params,
		stepMetadata:   stepMetadata,
		session:        session,
		tags:           tags,
		delegate:       delegate,
		tracker:        tracker,
	}
}

func (step dependentGetStep) Using(prev Step, repo *SourceRepository) Step {
	var info VersionInfo
	prev.Result(&info)

	return newGetStep(
		step.logger,
		step.sourceName,
		step.resourceConfig,
		info.Version,
		step.params,
		resource.ResourceCacheIdentifier{
			Type:    resource.ResourceType(step.resourceConfig.Type),
			Source:  step.resourceConfig.Source,
			Params:  step.params,
			Version: info.Version,
		},
		step.stepMetadata,
		step.session,
		step.tags,
		step.delegate,
		step.tracker,
	).Using(prev, repo)
}
