package exec

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/pivotal-golang/lager"
)

// DependentGetStep represents a Get step whose version is determined by the
// previous step. It is used to fetch the resource version produced by a
// PutStep.
type DependentGetStep struct {
	logger         lager.Logger
	sourceName     SourceName
	resourceConfig atc.ResourceConfig
	params         atc.Params
	stepMetadata   StepMetadata
	session        resource.Session
	tags           atc.Tags
	delegate       ResourceDelegate
	tracker        resource.Tracker
	resourceTypes  atc.ResourceTypes
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
	resourceTypes atc.ResourceTypes,
) DependentGetStep {
	return DependentGetStep{
		logger:         logger,
		sourceName:     sourceName,
		resourceConfig: resourceConfig,
		params:         params,
		stepMetadata:   stepMetadata,
		session:        session,
		tags:           tags,
		delegate:       delegate,
		tracker:        tracker,
		resourceTypes:  resourceTypes,
	}
}

// Using constructs a GetStep that will fetch the version of the resource
// determined by the VersionInfo result of the previous step.
func (step DependentGetStep) Using(prev Step, repo *SourceRepository) Step {
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
		step.resourceTypes,
	).Using(prev, repo)
}
