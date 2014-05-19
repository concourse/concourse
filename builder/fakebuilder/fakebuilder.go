package fakebuilder

import (
	"sync"

	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
)

type Builder struct {
	built       []config.Job
	builtMutex  *sync.Mutex
	BuildResult builds.Build
	BuildError  error
}

func New() *Builder {
	return &Builder{
		builtMutex: new(sync.Mutex),
	}
}

func (builder *Builder) Build(job config.Job) (builds.Build, error) {
	if builder.BuildError != nil {
		return builds.Build{}, builder.BuildError
	}

	builder.builtMutex.Lock()
	builder.built = append(builder.built, job)
	builder.builtMutex.Unlock()

	return builder.BuildResult, nil
}

func (builder *Builder) Built() []config.Job {
	return builder.built
}
