package exec

import (
	"io"

	"github.com/concourse/atc"
)

type Factory interface {
	Get(IOConfig, atc.ResourceConfig, atc.Params, atc.Version) Step
	Put(IOConfig, atc.ResourceConfig, atc.Params) Step
	// Delete(atc.ResourceConfig, atc.Params, atc.Version) Step
	Execute(IOConfig, BuildConfigSource) Step
}

type IOConfig struct {
	Stdout io.Writer
	Stderr io.Writer
}
