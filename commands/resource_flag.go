package commands

import (
	"strings"

	"github.com/concourse/fly/atcclient"
)

type ResourceFlag struct {
	PipelineName string
	ResourceName string
}

func (resource *ResourceFlag) UnmarshalFlag(value string) error {
	vs := strings.SplitN(value, "/", 2)
	if vs[0] == "" {
		return atcclient.NameRequiredError("pipeline")
	}
	if vs[1] == "" {
		return atcclient.NameRequiredError("resource")
	}

	resource.PipelineName = vs[0]
	resource.ResourceName = vs[1]

	return nil
}
