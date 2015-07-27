package factory

import (
	"errors"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

const defaultTaskName = "build"

type BuildFactory struct {
	PipelineName string
}

func (factory *BuildFactory) Create(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	inputs []db.BuildInput,
) (atc.Plan, error) {

	hasConditionals := factory.hasConditionals(job.Plan)
	hasHooks := factory.hasHooks(job.Plan)

	if hasHooks && hasConditionals {
		return atc.Plan{}, errors.New("you cannot have a plan with hooks and conditionals")
	}

	if hasConditionals {
		return factory.constructPlanSequenceBasedPlan(job.Plan, resources, inputs), nil
	} else {
		plan := factory.constructPlanHookBasedPlan(job.Plan, resources, inputs)
		return plan, nil
	}
}

func (factory *BuildFactory) hasConditionals(planSequence atc.PlanSequence) bool {
	return factory.doesAnyStepMatch(planSequence, func(step atc.PlanConfig) bool {
		return step.Conditions != nil
	})
}

func (factory *BuildFactory) hasHooks(planSequence atc.PlanSequence) bool {
	return factory.doesAnyStepMatch(planSequence, func(step atc.PlanConfig) bool {
		return step.Failure != nil || step.Ensure != nil || step.Success != nil
	})
}

func (factory *BuildFactory) doesAnyStepMatch(planSequence atc.PlanSequence, predicate func(step atc.PlanConfig) bool) bool {
	for _, planStep := range planSequence {
		if planStep.Aggregate != nil {
			if factory.doesAnyStepMatch(*planStep.Aggregate, predicate) {
				return true
			}
		}

		if planStep.Do != nil {
			if factory.doesAnyStepMatch(*planStep.Do, predicate) {
				return true
			}
		}

		if predicate(planStep) {
			return true
		}
	}

	return false
}

func (factory *BuildFactory) constructPlanHookBasedPlan(
	planSequence atc.PlanSequence,
	resources atc.ResourceConfigs,
	inputs []db.BuildInput,
) atc.Plan {
	if len(planSequence) == 0 {
		return atc.Plan{}
	}

	plan := factory.constructPlanFromConfig(
		planSequence[0],
		resources,
		inputs,
		true,
	)

	if len(planSequence) == 1 {
		return plan
	}

	if plan.HookedCompose != nil && (plan.HookedCompose.Next == atc.Plan{}) {
		plan.HookedCompose.Next = factory.constructPlanHookBasedPlan(planSequence[1:], resources, inputs)
		return plan
	} else {
		return atc.Plan{
			HookedCompose: &atc.HookedComposePlan{
				Step: plan,
				Next: factory.constructPlanHookBasedPlan(planSequence[1:], resources, inputs),
			},
		}
	}
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
		false,
	)

	for i := len(planSequence) - 1; i > 0; i-- {
		// plan preceding the current one in the sequence
		prevPlan := factory.constructPlanFromConfig(
			planSequence[i-1],
			resources,
			inputs,
			false,
		)

		// steps default to conditional on [success]
		plan = makeConditionalOnSuccess(plan)

		// if the previous plan is conditional, make the entire following chain
		// of composed steps conditional or get/put
		plan = conditionallyCompose(prevPlan, plan)
	}

	return plan
}

func makeConditionalOnSuccess(plan atc.Plan) atc.Plan {
	if plan.Conditional != nil {
		return plan
	} else if plan.Aggregate != nil {
		conditionaled := atc.AggregatePlan{}
		for _, plan := range *plan.Aggregate {
			conditionaled = append(conditionaled, makeConditionalOnSuccess(plan))
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
		if prevPlan.Conditional.Plan.PutGet != nil {
			plan = atc.Plan{
				Conditional: &atc.ConditionalPlan{
					Conditions: prevPlan.Conditional.Conditions,
					Plan: atc.Plan{
						PutGet: &atc.PutGetPlan{
							Head: prevPlan.Conditional.Plan.PutGet.Head,
							Rest: plan,
						},
					},
				},
			}
		} else {
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
		}
	} else {
		if prevPlan.PutGet != nil {
			plan = atc.Plan{
				PutGet: &atc.PutGetPlan{
					Head: prevPlan.PutGet.Head,
					Rest: plan,
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
	}

	return plan
}

func (factory *BuildFactory) constructPlanFromConfig(
	planConfig atc.PlanConfig,
	resources atc.ResourceConfigs,
	inputs []db.BuildInput,
	hasHooks bool,
) atc.Plan {
	var plan atc.Plan

	switch {
	case planConfig.Do != nil:
		if hasHooks {
			plan = factory.constructPlanHookBasedPlan(
				*planConfig.Do,
				resources,
				inputs,
			)
		} else {
			plan = factory.constructPlanSequenceBasedPlan(
				*planConfig.Do,
				resources,
				inputs,
			)
		}

	case planConfig.Put != "":
		logicalName := planConfig.Put

		resourceName := planConfig.Resource
		if resourceName == "" {
			resourceName = logicalName
		}

		resource, _ := resources.Lookup(resourceName)

		putPlan := &atc.PutPlan{
			Type:      resource.Type,
			Name:      logicalName,
			Pipeline:  factory.PipelineName,
			Resource:  resourceName,
			Source:    resource.Source,
			Params:    planConfig.Params,
			GetParams: planConfig.GetParams,
			Tags:      planConfig.Tags,
		}

		plan = atc.Plan{
			PutGet: &atc.PutGetPlan{
				Head: atc.Plan{
					Put: putPlan,
				},
				Rest: atc.Plan{},
			},
		}

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
				Pipeline: factory.PipelineName,
				Resource: resourceName,
				Source:   resource.Source,
				Params:   planConfig.Params,
				Version:  atc.Version(version),
				Tags:     planConfig.Tags,
			},
		}

	case planConfig.Task != "":
		plan = atc.Plan{
			Task: &atc.TaskPlan{
				Name:       planConfig.Task,
				Privileged: planConfig.Privileged,
				Config:     planConfig.TaskConfig,
				ConfigPath: planConfig.TaskConfigPath,
				Tags:       planConfig.Tags,
			},
		}

	case planConfig.Try != nil:
		plan = atc.Plan{
			Try: &atc.TryPlan{
				Step: factory.constructPlanFromConfig(
					*planConfig.Try,
					resources,
					inputs,
					hasHooks,
				),
			},
		}

	case planConfig.Aggregate != nil:
		aggregate := atc.AggregatePlan{}

		for _, planConfig := range *planConfig.Aggregate {
			aggregate = append(aggregate, factory.constructPlanFromConfig(
				planConfig,
				resources,
				inputs,
				hasHooks,
			))
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

	if planConfig.Timeout != "" {
		plan = atc.Plan{
			Timeout: &atc.TimeoutPlan{
				Duration: planConfig.Timeout,
				Step:     plan,
			},
		}
	}

	hooks := false
	failurePlan := atc.Plan{}

	if planConfig.Failure != nil {
		hooks = true
		failurePlan = factory.constructPlanFromConfig(*planConfig.Failure, resources, inputs, hasHooks)
	}

	ensurePlan := atc.Plan{}
	if planConfig.Ensure != nil {
		hooks = true
		ensurePlan = factory.constructPlanFromConfig(*planConfig.Ensure, resources, inputs, hasHooks)
	}

	successPlan := atc.Plan{}
	if planConfig.Success != nil {
		hooks = true
		successPlan = factory.constructPlanFromConfig(*planConfig.Success, resources, inputs, hasHooks)
	}

	if hooks {
		plan = atc.Plan{
			HookedCompose: &atc.HookedComposePlan{
				Step:         plan,
				OnFailure:    failurePlan,
				OnCompletion: ensurePlan,
				OnSuccess:    successPlan,
			},
		}
	}

	return plan
}
