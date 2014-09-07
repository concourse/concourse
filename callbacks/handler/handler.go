package handler

import (
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/logfanout"
)

type BuildDB interface {
	GetBuild(job string, build string) (builds.Build, error)
	SaveBuildStatus(job string, build string, status builds.Status) error

	SaveBuildInput(job string, build string, input builds.VersionedResource) error
	SaveBuildOutput(job string, build string, output builds.VersionedResource) error
}

type Handler struct {
	logger lager.Logger

	buildDB BuildDB
	tracker *logfanout.Tracker
}

func NewHandler(logger lager.Logger, buildDB BuildDB, tracker *logfanout.Tracker) *Handler {
	return &Handler{
		logger: logger,

		buildDB: buildDB,
		tracker: tracker,
	}
}
