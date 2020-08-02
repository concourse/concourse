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
	if warnings := atc.ValidateIdentifier(flag.ResourceName, "resource"); warnings != nil {
		return errors.New("argument format should be <pipeline>/<key:value>/<resource>")
	}

	vs := strings.SplitN(value[:resourceNameIdx], "/", 2)
	flag.PipelineRef.Name = vs[0]
	if len(vs) == 2 {
		instanceVars := atc.InstanceVars{}
		for _, instanceVar := range strings.Split(vs[1], ",") {
			kv := strings.SplitN(strings.TrimSpace(instanceVar), ":", 2)
			if len(kv) == 2 {
				instanceVars[kv[0]] = kv[1]
			} else {
				return errors.New("argument format should be <pipeline>/<key:value>/<resource>")
			}
		}
		flag.PipelineRef.InstanceVars = instanceVars
	}

	return nil
}
