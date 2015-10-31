package exec

import (
	"io"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Factory

type Factory interface {
	Get(lager.Logger, StepMetadata, SourceName, worker.Identifier, GetDelegate, atc.ResourceConfig, atc.Params, atc.Tags, atc.Version) StepFactory
	Put(lager.Logger, StepMetadata, worker.Identifier, PutDelegate, atc.ResourceConfig, atc.Tags, atc.Params) StepFactory
	DependentGet(lager.Logger, StepMetadata, SourceName, worker.Identifier, GetDelegate, atc.ResourceConfig, atc.Tags, atc.Params) StepFactory

	Task(lager.Logger, SourceName, worker.Identifier, TaskDelegate, Privileged, atc.Tags, TaskConfigSource) StepFactory
}

type StepMetadata interface {
	Env() []string
}

//go:generate counterfeiter . TaskDelegate

type TaskDelegate interface {
	Initializing(atc.TaskConfig)
	Started()

	Finished(ExitStatus)
	Failed(error)

	Stdout() io.Writer
	Stderr() io.Writer
}

type ResourceDelegate interface {
	Completed(ExitStatus, *VersionInfo)
	Failed(error)

	Stdout() io.Writer
	Stderr() io.Writer
}

//go:generate counterfeiter . GetDelegate

type GetDelegate interface {
	ResourceDelegate
}

//go:generate counterfeiter . PutDelegate

type PutDelegate interface {
	ResourceDelegate
}

type HijackedProcess interface {
	Wait() (int, error)
	SetTTY(atc.HijackTTYSpec) error
}

type Privileged bool
