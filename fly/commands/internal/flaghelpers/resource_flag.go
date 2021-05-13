package flaghelpers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/concourse/concourse/atc"
)

type ResourceFlag struct {
	PipelineRef  atc.PipelineRef
	ResourceName string
}

func (flag ResourceFlag) String() string {
	return fmt.Sprintf("%s/%s", flag.PipelineRef, flag.ResourceName)
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
		var err error
		flag.PipelineRef.InstanceVars, err = unmarshalInstanceVars(vs[1])
		if err != nil {
			return err
		}
	}

	return nil
}
