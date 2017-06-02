package exec

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

// DependentGetStep represents a Get step whose version is determined by the
// previous step. It is used to fetch the resource version produced by a
// PutStep.
type DependentGetStep struct {
	logger                 lager.Logger
	sourceName             worker.ArtifactName
	resourceConfig         atc.ResourceConfig
	params                 atc.Params
	stepMetadata           StepMetadata
	session                resource.Session
	tags                   atc.Tags
	teamID                 int
	buildID                int
	delegate               ResourceDelegate
	resourceFetcher        resource.Fetcher
	resourceTypes          atc.VersionedResourceTypes
	dbResourceCacheFactory db.ResourceCacheFactory
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
	buildID int,
	delegate ResourceDelegate,
	resourceFetcher resource.Fetcher,
	resourceTypes atc.VersionedResourceTypes,
	dbResourceCacheFactory db.ResourceCacheFactory,
) DependentGetStep {
	return DependentGetStep{
		logger:                 logger,
		sourceName:             sourceName,
		resourceConfig:         resourceConfig,
		params:                 params,
		stepMetadata:           stepMetadata,
		session:                session,
		tags:                   tags,
		teamID:                 teamID,
		buildID:                buildID,
		delegate:               delegate,
		resourceFetcher:        resourceFetcher,
		resourceTypes:          resourceTypes,
		dbResourceCacheFactory: dbResourceCacheFactory,
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
		resource.NewResourceInstance(
			resource.ResourceType(step.resourceConfig.Type),
			info.Version,
			step.resourceConfig.Source,
			step.params,
			db.ForBuild(step.buildID),
			step.resourceTypes,
			step.dbResourceCacheFactory,
		),
		step.stepMetadata,
		step.session,
		step.tags,
		step.teamID,
		step.delegate,
		step.resourceFetcher,
		step.resourceTypes,
	).Using(prev, repo)
}
