package exec

import (
	"io"

	"github.com/concourse/atc"
)

type SessionID string

//go:generate counterfeiter . Factory

type Factory interface {
	Get(SessionID, IOConfig, atc.ResourceConfig, atc.Params, atc.Version) Step
	Put(SessionID, IOConfig, atc.ResourceConfig, atc.Params) Step
	// Delete(atc.ResourceConfig, atc.Params, atc.Version) Step
	Execute(SessionID, IOConfig, Privileged, BuildConfigSource) Step

	Hijack(SessionID, IOConfig, atc.HijackProcessSpec) (HijackedProcess, error)
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
