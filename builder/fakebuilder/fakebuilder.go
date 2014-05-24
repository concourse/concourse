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
	Job               config.Job
	ResourceOverrides []config.Resource
}

func New() *Builder {
	return &Builder{
		builtMutex: new(sync.Mutex),
	}
}

func (builder *Builder) Build(job config.Job, resourceOverrides ...config.Resource) (builds.Build, error) {
	if builder.BuildError != nil {
		return builds.Build{}, builder.BuildError
	}

	builder.builtMutex.Lock()
	builder.built = append(builder.built, BuiltSpec{job, resourceOverrides})
	builder.builtMutex.Unlock()

	return builder.BuildResult, nil
}

func (builder *Builder) Built() []BuiltSpec {
	builder.builtMutex.Lock()
	defer builder.builtMutex.Unlock()

	return builder.built
}
