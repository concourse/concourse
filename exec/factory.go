package exec

import (
	"io"

	"github.com/concourse/atc"
)

type SessionID string

type Factory interface {
	Get(SessionID, IOConfig, atc.ResourceConfig, atc.Params, atc.Version) Step
	Put(SessionID, IOConfig, atc.ResourceConfig, atc.Params) Step
	// Delete(atc.ResourceConfig, atc.Params, atc.Version) Step
	Execute(SessionID, IOConfig, BuildConfigSource) Step
}

type IOConfig struct {
	Stdout io.Writer
	Stderr io.Writer
}
