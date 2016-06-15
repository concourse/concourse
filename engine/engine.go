package engine

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Engine

type Engine interface {
	Name() string

	CreateBuild(lager.Logger, db.Build, atc.Plan) (Build, error)
	LookupBuild(lager.Logger, db.Build) (Build, error)
}

//go:generate counterfeiter . EngineDB

type EngineDB interface {
	SaveBuildEvent(buildID int, pipelineID int, event atc.Event) error

	FinishBuild(buildID int, pipelineID int, status db.Status) error
	GetBuild(buildID int) (db.Build, bool, error)

	FindContainersByDescriptors(db.Container) ([]db.SavedContainer, error)
	FindLongLivedContainers(jobName string, pipelineID int) ([]db.SavedContainer, error)

	GetLatestFinishedBuild(jobID int) (db.Build, bool, error)

	SaveBuildEngineMetadata(buildID int, metadata string) error

	SaveBuildInput(buildID int, input db.BuildInput) (db.SavedVersionedResource, error)
	SaveBuildOutput(buildID int, vr db.VersionedResource, explicit bool) (db.SavedVersionedResource, error)

	SaveImageResourceVersion(buildID int, planID atc.PlanID, identifier db.ResourceCacheIdentifier) error

	GetPipelineByTeamNameAndName(teamName string, pipelineName string) (db.SavedPipeline, error)
}

//go:generate counterfeiter . Build

type Build interface {
	Metadata() string

	PublicPlan(lager.Logger) (atc.PublicBuildPlan, bool, error)

	Abort(lager.Logger) error
	Resume(lager.Logger)
}

type Engines []Engine

func (engines Engines) Lookup(name string) (Engine, bool) {
	for _, e := range engines {
		if e.Name() == name {
			return e, true
		}
	}

	return nil, false
}
