package exec

import "github.com/concourse/atc"

type Factory interface {
	Get(atc.ResourceConfig, atc.Params, atc.Version) Step
	Put(atc.ResourceConfig, atc.Params) Step
	// Delete(atc.ResourceConfig, atc.Params, atc.Version) Step
	Execute(BuildConfigSource) Step
}
