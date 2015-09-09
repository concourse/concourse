package config

import "github.com/concourse/atc"

// these are expressly tucked away so as to avoid accidental use in public API
// endpoints as that could leak credentials

type JobInput struct {
	Name     string
	Resource string
	Passed   []string
	Trigger  bool
	Params   atc.Params
	Tags     atc.Tags
}

type JobOutput struct {
	Name     string
	Resource string
}

func JobInputs(config atc.JobConfig) []JobInput {
	if config.InputConfigs != nil {
		var inputs []JobInput

		for _, config := range config.InputConfigs {
			inputs = append(inputs, JobInput{
				Name:     config.Name(),
				Resource: config.Resource,
				Passed:   config.Passed,
				Trigger:  config.Trigger,
			})
		}

		return inputs
	}

	return collectInputs(atc.PlanConfig{Do: &config.Plan})
}

func JobOutputs(config atc.JobConfig) []JobOutput {
	if config.OutputConfigs != nil {
		var outputs []JobOutput

		for _, config := range config.OutputConfigs {
			outputs = append(outputs, JobOutput{
				Name:     config.Resource,
				Resource: config.Resource,
			})
		}

		return outputs
	}

	return collectOutputs(atc.PlanConfig{Do: &config.Plan})
}

func collectInputs(plan atc.PlanConfig) []JobInput {
	var inputs []JobInput

	if plan.Success != nil {
		inputs = append(inputs, collectInputs(*plan.Success)...)
	}

	if plan.Failure != nil {
		inputs = append(inputs, collectInputs(*plan.Failure)...)
	}

	if plan.Ensure != nil {
		inputs = append(inputs, collectInputs(*plan.Ensure)...)
	}

	if plan.Try != nil {
		inputs = append(inputs, collectInputs(*plan.Try)...)
	}

	if plan.Do != nil {
		for _, p := range *plan.Do {
			inputs = append(inputs, collectInputs(p)...)
		}
	}

	if plan.Aggregate != nil {
		for _, p := range *plan.Aggregate {
			inputs = append(inputs, collectInputs(p)...)
		}
	}

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
			Trigger:  plan.Trigger,
			Params:   plan.Params,
			Tags:     plan.Tags,
		})
	}

	return inputs
}

func collectOutputs(plan atc.PlanConfig) []JobOutput {
	var outputs []JobOutput

	if plan.Success != nil {
		outputs = append(outputs, collectOutputs(*plan.Success)...)
	}

	if plan.Failure != nil {
		outputs = append(outputs, collectOutputs(*plan.Failure)...)
	}

	if plan.Ensure != nil {
		outputs = append(outputs, collectOutputs(*plan.Ensure)...)
	}

	if plan.Try != nil {
		outputs = append(outputs, collectOutputs(*plan.Try)...)
	}

	if plan.Do != nil {
		for _, p := range *plan.Do {
			outputs = append(outputs, collectOutputs(p)...)
		}
	}

	if plan.Aggregate != nil {
		var outputs []JobOutput

		for _, p := range *plan.Aggregate {
			outputs = append(outputs, collectOutputs(p)...)
		}

		return outputs
	}

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

	return outputs
}
