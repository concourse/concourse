package exec

import (
	"io"

	"github.com/concourse/atc"
)

type SessionID string

//go:generate counterfeiter . Factory

type Factory interface {
	Get(SessionID, GetDelegate, atc.ResourceConfig, atc.Params, atc.Version) Step
	Put(SessionID, PutDelegate, atc.ResourceConfig, atc.Params) Step
	// Delete(atc.ResourceConfig, atc.Params, atc.Version) Step
	Execute(SessionID, ExecuteDelegate, Privileged, BuildConfigSource) Step

	Hijack(SessionID, IOConfig, atc.HijackProcessSpec) (HijackedProcess, error)
}

//go:generate counterfeiter . ExecuteDelegate

type ExecuteDelegate interface {
	Initializing(atc.BuildConfig)
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
