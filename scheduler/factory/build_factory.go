package factory

import (
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

type BuildFactory struct {
	ConfigDB db.ConfigDB
}

func (factory *BuildFactory) Create(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	inputs []db.BuildInput,
) (atc.BuildPlan, error) {
	tInputs, err := factory.computeInputs(job, resources, inputs)
	if err != nil {
		return atc.BuildPlan{}, err
	}

	tOutputs, err := factory.computeOutputs(job, resources)
	if err != nil {
		return atc.BuildPlan{}, err
	}

	return atc.BuildPlan{
		Config:     job.BuildConfig,
		ConfigPath: job.BuildConfigPath,

		Inputs:  tInputs,
		Outputs: tOutputs,

		Privileged: job.Privileged,
	}, nil
}

func (factory *BuildFactory) computeInputs(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	dbInputs []db.BuildInput,
) ([]atc.InputPlan, error) {
	buildPlans := make([]atc.InputPlan, len(job.Inputs))
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
			if dbInput.Name == input.Name() {
				vr = dbInput.VersionedResource
			}
		}

		buildPlans[i] = factory.inputFor(job, vr, input.Name(), input.Params)
	}

	return buildPlans, nil
}

func (factory *BuildFactory) inputFor(
	job atc.JobConfig,
	vr db.VersionedResource,
	inputName string,
	params atc.Params,
) atc.InputPlan {
	inputPlan := atc.InputPlan{
		Name:     inputName,
		Resource: vr.Resource,
		Type:     vr.Type,
		Source:   atc.Source(vr.Source),
		Version:  atc.Version(vr.Version),
		Params:   atc.Params(params),
	}

	if inputPlan.Name == "" {
		inputPlan.Name = vr.Resource
	}

	return inputPlan
}

func (factory *BuildFactory) computeOutputs(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
) ([]atc.OutputPlan, error) {
	outputPlans := []atc.OutputPlan{}
	for _, output := range job.Outputs {
		resource, found := resources.Lookup(output.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", output.Resource)
		}

		outputPlan := atc.OutputPlan{
			Name:   resource.Name,
			Type:   resource.Type,
			On:     output.PerformOn(),
			Params: atc.Params(output.Params),
			Source: atc.Source(resource.Source),
		}

		outputPlans = append(outputPlans, outputPlan)
	}

	return outputPlans, nil
}
