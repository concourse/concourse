package flaghelpers

import (
	"errors"
	"strings"
)

type ResourceFlag struct {
	PipelineName string
	ResourceName string
}

func (resource *ResourceFlag) UnmarshalFlag(value string) error {
	vs := strings.SplitN(value, "/", 2)

	if len(vs) != 2 {
		return errors.New("argument format should be <pipeline>/<resource>")
	}

	resource.PipelineName = vs[0]
	resource.ResourceName = vs[1]

	return nil
}
