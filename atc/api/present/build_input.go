package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func BuildInput(input db.BuildInput, config atc.JobInput, source atc.Source) atc.BuildInput {
	return atc.BuildInput{
		Name:     input.Name,
		Resource: input.Resource,
		Type:     input.Type,
		Source:   source,
		Params:   config.Params,
		Version:  atc.Version(input.Version),
		Tags:     config.Tags,
	}
}
