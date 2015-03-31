package exec

import (
	"io"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . Factory

type Factory interface {
	Get(worker.Identifier, GetDelegate, atc.ResourceConfig, atc.Params, atc.Version) StepFactory
	Put(worker.Identifier, PutDelegate, atc.ResourceConfig, atc.Params) StepFactory
	// Delete(atc.ResourceConfig, atc.Params, atc.Version) Step
	Task(worker.Identifier, TaskDelegate, Privileged, TaskConfigSource) StepFactory
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
	Completed(VersionInfo)
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

type IOConfig struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}
