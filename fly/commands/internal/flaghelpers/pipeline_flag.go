package flaghelpers

import (
	"errors"
	"strings"

	"github.com/jessevdk/go-flags"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/rc"
)

type PipelineFlag struct {
	Name         string
	InstanceVars atc.InstanceVars
}

func (flag *PipelineFlag) Validate() error {

	if flag != nil {
		if strings.Contains(flag.Name, "/") {
			return errors.New("pipeline name cannot contain '/'")
		}

		configError := atc.ValidateIdentifier(flag.Name, "pipeline")
		if configError != nil {
			return configError
		}

	}
	return nil
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
		var err error
		flag.InstanceVars, err = unmarshalInstanceVars(vs[1])
		if err != nil {
			return err
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
