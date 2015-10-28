package commands

import (
	"strings"

	"github.com/concourse/go-concourse/concourse"
)

type ResourceFlag struct {
	PipelineName string
	ResourceName string
}

func (resource *ResourceFlag) UnmarshalFlag(value string) error {
	vs := strings.SplitN(value, "/", 2)
	if vs[0] == "" {
		return concourse.NameRequiredError("pipeline")
	}
	if vs[1] == "" {
		return concourse.NameRequiredError("resource")
	}

	resource.PipelineName = vs[0]
	resource.ResourceName = vs[1]

	return nil
}
