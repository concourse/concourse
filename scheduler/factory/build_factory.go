package factory

import (
	"fmt"
	"path/filepath"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/turbine"
)

type BuildFactory struct {
	ConfigDB ConfigDB
}

type ConfigDB interface {
	GetConfig() (atc.Config, error)
}

func (factory *BuildFactory) Create(job atc.JobConfig, inputVersions db.VersionedResources) (turbine.Build, error) {
	config, err := factory.ConfigDB.GetConfig()
	if err != nil {
		return turbine.Build{}, err
	}

	inputs, err := factory.computeInputs(job, config.Resources, inputVersions)
	if err != nil {
		return turbine.Build{}, err
	}

	outputs, err := factory.computeOutputs(job, config.Resources)
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

func (factory *BuildFactory) computeInputs(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	inputs db.VersionedResources,
) ([]turbine.Input, error) {
	turbineInputs := make([]turbine.Input, len(job.Inputs))
	for i, input := range job.Inputs {
		resource, found := resources.Lookup(input.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", input.Resource)
		}

		vr, found := inputs.Lookup(input.Resource)
		if !found {
			vr = db.VersionedResource{
				Name:   resource.Name,
				Type:   resource.Type,
				Source: db.Source(resource.Source),
			}
		}

		turbineInputs[i] = factory.inputFor(job, vr, input.Name, input.Params)
	}

	return turbineInputs, nil
}

func (factory *BuildFactory) inputFor(
	job atc.JobConfig,
	vr db.VersionedResource,
	inputName string,
	params atc.Params,
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

func (factory *BuildFactory) computeOutputs(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
) ([]turbine.Output, error) {
	turbineOutputs := []turbine.Output{}
	for _, output := range job.Outputs {
		resource, found := resources.Lookup(output.Resource)
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
