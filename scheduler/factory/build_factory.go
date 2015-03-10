package factory

import (
	"errors"
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

const defaultExecuteName = "build"

type BuildFactory struct {
	ConfigDB db.ConfigDB
}

var ErrBothPlanAndIOConfigured = errors.New("both plan and inputs/outputs configured")

func (factory *BuildFactory) Create(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	inputs []db.BuildInput,
) (atc.Plan, error) {
	if factory.hasPlanConfig(job) && factory.hasIOConfig(job) {
		return atc.Plan{}, ErrBothPlanAndIOConfigured
	}

	if factory.hasIOConfig(job) {
		return factory.constructIOBasedPlan(job, resources, inputs)
	} else if factory.hasPlanConfig(job) {
		return factory.constructPlanSequenceBasedPlan(job.Plan, resources, inputs), nil
	}

	return atc.Plan{}, nil
}

func (factory *BuildFactory) constructPlanSequenceBasedPlan(
	planSequence atc.PlanSequence,
	resources atc.ResourceConfigs,
	inputs []db.BuildInput,
) atc.Plan {
	if len(planSequence) == 0 {
		return atc.Plan{}
	}

	// work backwards to simplify conditional wrapping
	plan := factory.constructPlanFromConfig(
		planSequence[len(planSequence)-1],
		resources,
		inputs,
	)

	for i := len(planSequence) - 1; i > 0; i-- {
		// plan preceding the current one in the sequence
		prevPlan := factory.constructPlanFromConfig(
			planSequence[i-1],
			resources,
			inputs,
		)

		// if following an execute step, later steps default to on [success]
		if prevPlan.Execute != nil && plan.Conditional == nil {
			plan = makeConditionalOnSuccess(plan)
		}

		// if the previous plan is conditional, make the entire following sequence
		// conditional
		plan = conditionallyCompose(prevPlan, plan)
	}

	return plan
}

func makeConditionalOnSuccess(plan atc.Plan) atc.Plan {
	if plan.Aggregate != nil {
		conditionaled := atc.AggregatePlan{}
		for name, plan := range *plan.Aggregate {
			if plan.Conditional == nil {
				plan = atc.Plan{
					Conditional: &atc.ConditionalPlan{
						Conditions: atc.Conditions{atc.ConditionSuccess},
						Plan:       plan,
					},
				}
			}

			conditionaled[name] = plan
		}

		plan.Aggregate = &conditionaled
	} else {
		plan = atc.Plan{
			Conditional: &atc.ConditionalPlan{
				Conditions: atc.Conditions{atc.ConditionSuccess},
				Plan:       plan,
			},
		}
	}

	return plan
}

func conditionallyCompose(prevPlan atc.Plan, plan atc.Plan) atc.Plan {
	if prevPlan.Conditional != nil {
		plan = atc.Plan{
			Conditional: &atc.ConditionalPlan{
				Conditions: prevPlan.Conditional.Conditions,
				Plan: atc.Plan{
					Compose: &atc.ComposePlan{
						A: prevPlan.Conditional.Plan,
						B: plan,
					},
				},
			},
		}
	} else {
		plan = atc.Plan{
			Compose: &atc.ComposePlan{
				A: prevPlan,
				B: plan,
			},
		}
	}

	return plan
}

func (factory *BuildFactory) constructPlanFromConfig(
	planConfig atc.PlanConfig,
	resources atc.ResourceConfigs,
	inputs []db.BuildInput,
) atc.Plan {
	var plan atc.Plan

	switch {
	case planConfig.Do != nil:
		plan = factory.constructPlanSequenceBasedPlan(
			*planConfig.Do,
			resources,
			inputs,
		)

	case planConfig.Get != "":
		resourceName := planConfig.Resource
		if resourceName == "" {
			resourceName = planConfig.Get
		}

		resource, _ := resources.Lookup(resourceName)

		name := planConfig.Get
		var version db.Version
		for _, input := range inputs {
			if input.Name == name {
				version = input.Version
				break
			}
		}

		plan = atc.Plan{
			Get: &atc.GetPlan{
				Type:     resource.Type,
				Name:     name,
				Resource: resourceName,
				Source:   resource.Source,
				Params:   planConfig.Params,
				Version:  atc.Version(version),
			},
		}

	case planConfig.Put != "":
		resourceName := planConfig.Resource
		if resourceName == "" {
			resourceName = planConfig.Put
		}

		resource, _ := resources.Lookup(resourceName)

		plan = atc.Plan{
			Put: &atc.PutPlan{
				Type:     resource.Type,
				Name:     planConfig.Put,
				Resource: resourceName,
				Source:   resource.Source,
				Params:   planConfig.Params,
			},
		}

	case planConfig.Execute != "":
		plan = atc.Plan{
			Execute: &atc.ExecutePlan{
				Name:       planConfig.Execute,
				Privileged: planConfig.Privileged,
				Config:     planConfig.BuildConfig,
				ConfigPath: planConfig.BuildConfigPath,
			},
		}

	case planConfig.Aggregate != nil:
		aggregate := atc.AggregatePlan{}

		for _, planConfig := range *planConfig.Aggregate {
			aggregate[planConfig.Name()] = factory.constructPlanFromConfig(
				planConfig,
				resources,
				inputs,
			)
		}

		plan = atc.Plan{
			Aggregate: &aggregate,
		}
	}

	if planConfig.Conditions != nil {
		plan = atc.Plan{
			Conditional: &atc.ConditionalPlan{
				Conditions: *planConfig.Conditions,
				Plan:       plan,
			},
		}
	}

	return plan
}

func (factory *BuildFactory) constructIOBasedPlan(
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
							Name: defaultExecuteName,

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

func (factory *BuildFactory) hasPlanConfig(job atc.JobConfig) bool {
	return len(job.Plan) > 0
}

func (factory *BuildFactory) hasIOConfig(job atc.JobConfig) bool {
	return len(job.InputConfigs) > 0 ||
		len(job.OutputConfigs) > 0 ||
		job.BuildConfig != nil ||
		len(job.BuildConfigPath) > 0
}

func (factory *BuildFactory) computeInputs(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	dbInputs []db.BuildInput,
) (atc.AggregatePlan, error) {
	getPlans := atc.AggregatePlan{}

	for _, input := range job.InputConfigs {
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

	for _, output := range job.OutputConfigs {
		resource, found := resources.Lookup(output.Resource)
		if !found {
			return nil, fmt.Errorf("unknown resource: %s", output.Resource)
		}

		plan := atc.Plan{
			Put: &atc.PutPlan{
				Name:     resource.Name,
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
