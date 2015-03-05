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
) (atc.Plan, error) {
	tInputs, err := factory.computeInputs(job, resources, inputs)
	if err != nil {
		return atc.Plan{}, err
	}

	tOutputs, err := factory.computeOutputs(job, resources)
	if err != nil {
		return atc.Plan{}, err
	}

	return atc.Plan{
		Compose: &atc.ComposePlan{
			A: atc.Plan{
				Aggregate: &tInputs,
			},
			B: atc.Plan{
				Compose: &atc.ComposePlan{
					A: atc.Plan{
						Execute: &atc.ExecutePlan{
							Privileged: job.Privileged,

							Config:     job.BuildConfig,
							ConfigPath: job.BuildConfigPath,
						},
					},
					B: atc.Plan{
						Aggregate: &tOutputs,
					},
				},
			},
		},
	}, nil
}

func (factory *BuildFactory) computeInputs(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	dbInputs []db.BuildInput,
) (atc.AggregatePlan, error) {
	getPlans := atc.AggregatePlan{}

	for _, input := range job.Inputs {
		resource, found := resources.Lookup(input.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", input.Resource)
		}

		getPlan := atc.GetPlan{
			Name:     input.Name(),
			Resource: resource.Name,
			Type:     resource.Type,
			Source:   atc.Source(resource.Source),
			Params:   atc.Params(input.Params),
		}

		for _, dbInput := range dbInputs {
			vr := dbInput.VersionedResource

			if dbInput.Name == getPlan.Name {
				getPlan.Type = vr.Type
				getPlan.Source = atc.Source(vr.Source)
				getPlan.Version = atc.Version(vr.Version)
				break
			}
		}

		getPlans[input.Name()] = atc.Plan{
			Get: &getPlan,
		}
	}

	return getPlans, nil
}

func (factory *BuildFactory) computeOutputs(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
) (atc.AggregatePlan, error) {
	outputPlans := atc.AggregatePlan{}

	for _, output := range job.Outputs {
		resource, found := resources.Lookup(output.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", output.Resource)
		}

		plan := atc.Plan{
			Put: &atc.PutPlan{
				Resource: resource.Name,
				Type:     resource.Type,
				Params:   atc.Params(output.Params),
				Source:   atc.Source(resource.Source),
			},
		}

		if job.BuildConfig != nil || job.BuildConfigPath != "" {
			plan = atc.Plan{
				Conditional: &atc.ConditionalPlan{
					Conditions: output.PerformOn(),
					Plan:       plan,
				},
			}
		}

		outputPlans[output.Resource] = plan
	}

	return outputPlans, nil
}
