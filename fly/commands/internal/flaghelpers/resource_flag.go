package flaghelpers

import (
	"errors"
	"strings"

	"github.com/concourse/concourse/atc"
)

type ResourceFlag struct {
	PipelineRef  atc.PipelineRef
	ResourceName string
}

func (flag *ResourceFlag) UnmarshalFlag(value string) error {
	flag.PipelineRef = atc.PipelineRef{}

	resourceNameIdx := strings.LastIndex(value, "/")
	if resourceNameIdx == -1 {
		return errors.New("argument format should be <pipeline>/<resource>")
	}

	flag.ResourceName = value[resourceNameIdx+1:]
	if flag.ResourceName == "" {
		return errors.New("argument format should be <pipeline>/<resource>")
	}

	vs := strings.SplitN(value[:resourceNameIdx], "/", 2)
	flag.PipelineRef.Name = vs[0]
	if len(vs) == 2 {
		flatInstanceVars, err := unmarshalDotNotation(vs[1])
		if err != nil {
			return errors.New(err.Error() + "/<resource>")
		}
		flag.PipelineRef.InstanceVars, err = flatInstanceVars.Expand()
		if err != nil {
			return err
		}
	}

	return nil
}
