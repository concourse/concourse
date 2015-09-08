package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func BuildInput(input db.BuildInput) atc.BuildInput {
	return atc.BuildInput{
		Name:     input.Name,
		Resource: input.Resource,
		Type:     input.Type,
		Source:   atc.Source(input.Source),
		Version:  atc.Version(input.Version),
	}
}
