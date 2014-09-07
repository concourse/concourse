package factory

import (
	"fmt"
	"path/filepath"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	tbuilds "github.com/concourse/turbine/api/builds"
)

type BuildFactory struct {
	Resources config.Resources
}

func (factory *BuildFactory) Create(job config.Job, inputVersions builds.VersionedResources) (tbuilds.Build, error) {
	inputs, err := factory.computeInputs(job, inputVersions)
	if err != nil {
		return tbuilds.Build{}, err
	}

	outputs, err := factory.computeOutputs(job)
	if err != nil {
		return tbuilds.Build{}, err
	}

	return tbuilds.Build{
		Config: job.BuildConfig,

		Inputs:  inputs,
		Outputs: outputs,

		Privileged: job.Privileged,
	}, nil
}

func (factory *BuildFactory) computeInputs(job config.Job, inputs builds.VersionedResources) ([]tbuilds.Input, error) {
	turbineInputs := make([]tbuilds.Input, len(job.Inputs))
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

		turbineInputs[i] = factory.inputFor(job, vr, input.Params)
	}

	return turbineInputs, nil
}

func (factory *BuildFactory) inputFor(job config.Job, vr builds.VersionedResource, params config.Params) tbuilds.Input {
	turbineInput := tbuilds.Input{
		Name:    vr.Name,
		Type:    vr.Type,
		Source:  tbuilds.Source(vr.Source),
		Version: tbuilds.Version(vr.Version),
		Params:  tbuilds.Params(params),
	}

	if filepath.HasPrefix(job.BuildConfigPath, vr.Name) {
		turbineInput.ConfigPath = job.BuildConfigPath[len(vr.Name)+1:]
	}

	return turbineInput
}

func (factory *BuildFactory) computeOutputs(job config.Job) ([]tbuilds.Output, error) {
	turbineOutputs := []tbuilds.Output{}
	for _, output := range job.Outputs {
		resource, found := factory.Resources.Lookup(output.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", output.Resource)
		}

		conditions := []tbuilds.OutputCondition{}

		// if not specified, assume [success]
		//
		// note that this check is for nil, not len(output.On) == 0
		if output.On == nil {
			conditions = append(conditions, tbuilds.OutputConditionSuccess)
		} else {
			for _, cond := range output.On {
				conditions = append(conditions, tbuilds.OutputCondition(cond))
			}
		}

		turbineOutput := tbuilds.Output{
			Name:   resource.Name,
			Type:   resource.Type,
			On:     conditions,
			Params: tbuilds.Params(output.Params),
			Source: tbuilds.Source(resource.Source),
		}

		turbineOutputs = append(turbineOutputs, turbineOutput)
	}

	return turbineOutputs, nil
}
