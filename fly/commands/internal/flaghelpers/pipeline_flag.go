package flaghelpers

import (
	"errors"
	"strings"

	"github.com/jessevdk/go-flags"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/go-concourse/concourse"
)

type PipelineFlag struct {
	Name         string
	InstanceVars atc.InstanceVars
}

func (flag *PipelineFlag) Validate() ([]concourse.ConfigWarning, error) {
	var warnings []concourse.ConfigWarning
	if flag != nil {
		if strings.Contains(flag.Name, "/") {
			return nil, errors.New("pipeline name cannot contain '/'")
		}

		if warning := atc.ValidateIdentifier(flag.Name, "pipeline"); warning != nil {
			warnings = append(warnings, concourse.ConfigWarning{
				Type:    warning.Type,
				Message: warning.Message,
			})
		}
	}
	return warnings, nil
}

func (flag *PipelineFlag) Ref() atc.PipelineRef {
	return atc.PipelineRef{Name: flag.Name, InstanceVars: flag.InstanceVars}
}

func (flag *PipelineFlag) UnmarshalFlag(value string) error {
	if !strings.Contains(value, "/") {
		flag.Name = value
		return nil
	}

	vs := strings.SplitN(value, "/", 2)
	if len(vs) == 2 {
		flag.Name = vs[0]
		flag.InstanceVars = atc.InstanceVars{}
		instanceVars := strings.Split(vs[1], ",")
		for _, instanceVar := range instanceVars {
			kv := strings.SplitN(strings.TrimSpace(instanceVar), ":", 2)
			if len(kv) == 2 {
				flag.InstanceVars[kv[0]] = kv[1]
			} else {
				return errors.New("argument format should be <pipeline>/<key:value>")
			}
		}
	}

	return nil
}

func (flag *PipelineFlag) Complete(match string) []flags.Completion {
	fly := parseFlags()

	target, err := rc.LoadTarget(fly.Target, false)
	if err != nil {
		return []flags.Completion{}
	}

	err = target.Validate()
	if err != nil {
		return []flags.Completion{}
	}

	pipelines, err := target.Team().ListPipelines()
	if err != nil {
		return []flags.Completion{}
	}

	comps := []flags.Completion{}
	for _, pipeline := range pipelines {
		if strings.HasPrefix(pipeline.Ref().String(), match) {
			comps = append(comps, flags.Completion{Item: pipeline.Ref().String()})
		}
	}

	return comps
}
