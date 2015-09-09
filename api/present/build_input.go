package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func BuildInput(input db.BuildInput, source atc.Source) atc.BuildInput {
	return atc.BuildInput{
		Name:     input.Name,
		Resource: input.Resource,
		Type:     input.Type,
		Source:   source,
		Version:  atc.Version(input.Version),
	}
}
