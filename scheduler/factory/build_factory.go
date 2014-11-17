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

func (factory *BuildFactory) Create(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	inputs []db.BuildInput,
) (turbine.Build, error) {
	tInputs, err := factory.computeInputs(job, resources, inputs)
	if err != nil {
		return turbine.Build{}, err
	}

	tOutputs, err := factory.computeOutputs(job, resources)
	if err != nil {
		return turbine.Build{}, err
	}

	return turbine.Build{
		Config: job.BuildConfig,

		Inputs:  tInputs,
		Outputs: tOutputs,

		Privileged: job.Privileged,
	}, nil
}

func (factory *BuildFactory) computeInputs(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	dbInputs []db.BuildInput,
) ([]turbine.Input, error) {
	turbineInputs := make([]turbine.Input, len(job.Inputs))
	for i, input := range job.Inputs {
		resource, found := resources.Lookup(input.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", input.Resource)
		}

		vr := db.VersionedResource{
			Resource: resource.Name,
			Type:     resource.Type,
			Source:   db.Source(resource.Source),
		}

		for _, dbInput := range dbInputs {
			if dbInput.Name == input.Name {
				vr = dbInput.VersionedResource
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
		Resource: vr.Resource,
		Type:     vr.Type,
		Source:   turbine.Source(vr.Source),
		Version:  turbine.Version(vr.Version),
		Params:   turbine.Params(params),
	}

	if turbineInput.Name == "" {
		turbineInput.Name = vr.Resource
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
		for _, cond := range output.PerformOn {
			conditions = append(conditions, turbine.OutputCondition(cond))
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
