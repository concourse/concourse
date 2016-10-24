package exec

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

// DependentGetStep represents a Get step whose version is determined by the
// previous step. It is used to fetch the resource version produced by a
// PutStep.
type DependentGetStep struct {
	logger              lager.Logger
	sourceName          worker.ArtifactName
	resourceConfig      atc.ResourceConfig
	params              atc.Params
	stepMetadata        StepMetadata
	session             resource.Session
	tags                atc.Tags
	teamID              int
	delegate            ResourceDelegate
	resourceFetcher     resource.Fetcher
	resourceTypes       atc.ResourceTypes
	containerSuccessTTL time.Duration
	containerFailureTTL time.Duration
}

func newDependentGetStep(
	logger lager.Logger,
	sourceName worker.ArtifactName,
	resourceConfig atc.ResourceConfig,
	params atc.Params,
	stepMetadata StepMetadata,
	session resource.Session,
	tags atc.Tags,
	teamID int,
	delegate ResourceDelegate,
	resourceFetcher resource.Fetcher,
	resourceTypes atc.ResourceTypes,
	containerSuccessTTL time.Duration,
	containerFailureTTL time.Duration,
) DependentGetStep {
	return DependentGetStep{
		logger:              logger,
		sourceName:          sourceName,
		resourceConfig:      resourceConfig,
		params:              params,
		stepMetadata:        stepMetadata,
		session:             session,
		tags:                tags,
		teamID:              teamID,
		delegate:            delegate,
		resourceFetcher:     resourceFetcher,
		resourceTypes:       resourceTypes,
		containerSuccessTTL: containerSuccessTTL,
		containerFailureTTL: containerFailureTTL,
	}
}

// Using constructs a GetStep that will fetch the version of the resource
// determined by the VersionInfo result of the previous step.
func (step DependentGetStep) Using(prev Step, repo *worker.ArtifactRepository) Step {
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
		step.teamID,
		step.delegate,
		step.resourceFetcher,
		step.resourceTypes,
		step.containerSuccessTTL,
		step.containerFailureTTL,
	).Using(prev, repo)
}
