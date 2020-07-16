package flaghelpers

import (
	"errors"
	"strings"

	"github.com/jessevdk/go-flags"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/go-concourse/concourse"
)

type PipelineFlag string

func (flag *PipelineFlag) Validate() ([]concourse.ConfigWarning, error) {
	if strings.Contains(string(*flag), "/") {
		return nil, errors.New("pipeline name cannot contain '/'")
	}

	var warnings []concourse.ConfigWarning
	if warning := atc.ValidateIdentifier(string(*flag), "pipeline"); warning != nil {
		warnings = append(warnings, concourse.ConfigWarning{
			Type:    warning.Type,
			Message: warning.Message,
		})
	}
	return warnings, nil
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
