package flaghelpers

import (
	"errors"
	"strings"

	"github.com/jessevdk/go-flags"

	"github.com/concourse/fly/rc"
)

type PipelineFlag string

func (flag *PipelineFlag) Validate() error {
	if strings.Contains(string(*flag), "/") {
		return errors.New("pipeline name cannot contain '/'")
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
		if strings.HasPrefix(pipeline.Name, match) {
			comps = append(comps, flags.Completion{Item: pipeline.Name})
		}
	}

	return comps
}
