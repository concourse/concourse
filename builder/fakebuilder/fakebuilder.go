package fakebuilder

import (
	"sync"

	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
)

type BuildFunc func(config.Job, map[string]builds.Version) (builds.Build, error)

type Builder struct {
	built        []BuiltSpec
	builtMutex   *sync.Mutex
	WhenBuilding BuildFunc
	BuildResult  builds.Build
	BuildError   error
}

type BuiltSpec struct {
	Job              config.Job
	VersionOverrides map[string]builds.Version
}

func New() *Builder {
	return &Builder{
		builtMutex: new(sync.Mutex),
	}
}

func (builder *Builder) Build(job config.Job, versionOverrides map[string]builds.Version) (builds.Build, error) {
	if builder.BuildError != nil {
		return builds.Build{}, builder.BuildError
	}

	if builder.WhenBuilding != nil {
		return builder.WhenBuilding(job, versionOverrides)
	}

	builder.builtMutex.Lock()
	builder.built = append(builder.built, BuiltSpec{job, versionOverrides})
	builder.builtMutex.Unlock()

	return builder.BuildResult, nil
}

func (builder *Builder) Built() []BuiltSpec {
	builder.builtMutex.Lock()
	defer builder.builtMutex.Unlock()

	return builder.built
}
