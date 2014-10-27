package factory

import (
	"fmt"
	"path/filepath"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/turbine"
)

type BuildFactory struct {
	Resources config.Resources
}

func (factory *BuildFactory) Create(job config.Job, inputVersions builds.VersionedResources) (turbine.Build, error) {
	inputs, err := factory.computeInputs(job, inputVersions)
	if err != nil {
		return turbine.Build{}, err
	}

	outputs, err := factory.computeOutputs(job)
	if err != nil {
		return turbine.Build{}, err
	}

	return turbine.Build{
		Config: job.BuildConfig,

		Inputs:  inputs,
		Outputs: outputs,

		Privileged: job.Privileged,
	}, nil
}

func (factory *BuildFactory) computeInputs(job config.Job, inputs builds.VersionedResources) ([]turbine.Input, error) {
	turbineInputs := make([]turbine.Input, len(job.Inputs))
	for i, input := range job.Inputs {
		resource, found := factory.Resources.Lookup(input.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", input.Resource)
		}

		vr, found := inputs.Lookup(input.Resource)
		if !found {
			vr = builds.VersionedResource{
				Name:   resource.Name,
				Type:   resource.Type,
				Source: builds.Source(resource.Source),
			}
		}

		turbineInputs[i] = factory.inputFor(job, vr, input.Name, input.Params)
	}

	return turbineInputs, nil
}

func (factory *BuildFactory) inputFor(
	job config.Job,
	vr builds.VersionedResource,
	inputName string,
	params config.Params,
) turbine.Input {
	turbineInput := turbine.Input{
		Name:     inputName,
		Resource: vr.Name,
		Type:     vr.Type,
		Source:   turbine.Source(vr.Source),
		Version:  turbine.Version(vr.Version),
		Params:   turbine.Params(params),
	}

	if turbineInput.Name == "" {
		turbineInput.Name = vr.Name
	}

	if filepath.HasPrefix(job.BuildConfigPath, turbineInput.Name+"/") {
		turbineInput.ConfigPath = job.BuildConfigPath[len(turbineInput.Name)+1:]
	}

	return turbineInput
}

func (factory *BuildFactory) computeOutputs(job config.Job) ([]turbine.Output, error) {
	turbineOutputs := []turbine.Output{}
	for _, output := range job.Outputs {
		resource, found := factory.Resources.Lookup(output.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", output.Resource)
		}

		conditions := []turbine.OutputCondition{}

		// if not specified, assume [success]
		//
		// note that this check is for nil, not len(output.PerformOn) == 0
		if output.PerformOn == nil {
			conditions = append(conditions, turbine.OutputConditionSuccess)
		} else {
			for _, cond := range output.PerformOn {
				conditions = append(conditions, turbine.OutputCondition(cond))
			}
		}

		turbineOutput := turbine.Output{
			Name:   resource.Name,
			Type:   resource.Type,
			On:     conditions,
			Params: turbine.Params(output.Params),
			Source: turbine.Source(resource.Source),
		}

		turbineOutputs = append(turbineOutputs, turbineOutput)
	}

	return turbineOutputs, nil
}
