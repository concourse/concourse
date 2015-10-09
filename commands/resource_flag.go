package commands

import "strings"

type ResourceFlag struct {
	PipelineName string
	ResourceName string
}

func (resource *ResourceFlag) UnmarshalFlag(value string) error {
	vs := strings.SplitN(value, "/", 2)
	if len(vs) != 2 {
		resource.PipelineName = "main"
		resource.ResourceName = vs[0]
	} else {
		resource.PipelineName = vs[0]
		resource.ResourceName = vs[1]
	}

	return nil
}
