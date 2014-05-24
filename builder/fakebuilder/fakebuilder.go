package fakebuilder

import (
	"sync"

	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
)

type Builder struct {
	built       []BuiltSpec
	builtMutex  *sync.Mutex
	BuildResult builds.Build
	BuildError  error
}

type BuiltSpec struct {
	Job       config.Job
	Resources config.Resources
}

func New() *Builder {
	return &Builder{
		builtMutex: new(sync.Mutex),
	}
}

func (builder *Builder) Build(job config.Job, resources config.Resources) (builds.Build, error) {
	if builder.BuildError != nil {
		return builds.Build{}, builder.BuildError
	}

	builder.builtMutex.Lock()
	builder.built = append(builder.built, BuiltSpec{job, resources})
	builder.builtMutex.Unlock()

	return builder.BuildResult, nil
}

func (builder *Builder) Built() []BuiltSpec {
	return builder.built
}
