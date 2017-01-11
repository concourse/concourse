package config

import "github.com/concourse/atc"

// these are expressly tucked away so as to avoid accidental use in public API
// endpoints as that could leak credentials

type JobInput struct {
	Name     string
	Resource string
	Passed   []string
	Trigger  bool
	Version  *atc.VersionConfig
	Params   atc.Params
	Tags     atc.Tags
}

type JobOutput struct {
	Name     string
	Resource string
}

func JobInputs(config atc.JobConfig) []JobInput {
	var inputs []JobInput

	for _, plan := range config.Plans() {
		if plan.Get != "" {
			get := plan.Get

			resource := get
			if plan.Resource != "" {
				resource = plan.Resource
			}

			inputs = append(inputs, JobInput{
				Name:     get,
				Resource: resource,
				Passed:   plan.Passed,
				Version:  plan.Version,
				Trigger:  plan.Trigger,
				Params:   plan.Params,
				Tags:     plan.Tags,
			})
		}
	}

	return inputs
}

func JobOutputs(config atc.JobConfig) []JobOutput {
	var outputs []JobOutput

	for _, plan := range config.Plans() {
		if plan.Put != "" {
			put := plan.Put

			resource := put
			if plan.Resource != "" {
				resource = plan.Resource
			}

			outputs = append(outputs, JobOutput{
				Name:     put,
				Resource: resource,
			})
		}
	}

	return outputs
}
