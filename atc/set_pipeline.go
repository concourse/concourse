package atc

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

type SetPipelineParamsXX struct {
	PipelineName   string                 `json:"pipeline_name"`
	Config         string                 `json:"config"`
	LoadVarsFrom   []string               `json:"load_vars_from,omitempty"`
	Var            map[string]interface{} `json:"var,omitempty"`
	FailWhenNoDiff bool                   `json:"fail_when_no_diff,omitempty"`
}

func NewSetPipelineParams(params Params) (SetPipelineParamsXX, error) {
	bytes, err := yaml.Marshal(params)
	if err != nil {
		return SetPipelineParamsXX{}, err
	}

	var spParams SetPipelineParamsXX
	err = yaml.UnmarshalStrict(bytes, &spParams, yaml.DisallowUnknownFields)
	if err != nil {
		return SetPipelineParamsXX{}, err
	}

	err = spParams.Validate()
	if err != nil {
		return SetPipelineParamsXX{}, err
	}

	return spParams, nil
}

func (config SetPipelineParamsXX) Validate() error {
	var messages []string

	if config.PipelineName == "" {
		messages = append(messages, "  missing 'pipeline_name'")
	}

	if config.Config == "" {
		messages = append(messages, "  missing 'config'")
	}

	if len(messages) > 0 {
		return fmt.Errorf("invalid task configuration:\n%s", strings.Join(messages, "\n"))
	}

	return nil
}

