package handler

import (
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/logfanout"
)

type BuildDB interface {
	GetBuild(build int) (builds.Build, error)
	SaveBuildStatus(build int, status builds.Status) error
	SaveBuildInput(build int, input builds.VersionedResource) error
	SaveBuildOutput(build int, output builds.VersionedResource) error
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
