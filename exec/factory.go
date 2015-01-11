package exec

import (
	"io"

	garden "github.com/cloudfoundry-incubator/garden/api"
	"github.com/concourse/atc"
)

type SessionID string

//go:generate counterfeiter . Factory

type Factory interface {
	Get(SessionID, IOConfig, atc.ResourceConfig, atc.Params, atc.Version) Step
	Put(SessionID, IOConfig, atc.ResourceConfig, atc.Params) Step
	// Delete(atc.ResourceConfig, atc.Params, atc.Version) Step
	Execute(SessionID, IOConfig, Privileged, BuildConfigSource) Step

	Hijack(SessionID, garden.ProcessSpec, garden.ProcessIO) (garden.Process, error)
}

type Privileged bool

type IOConfig struct {
	Stdout io.Writer
	Stderr io.Writer
}
