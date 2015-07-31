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
		return factory.constructPlanSequenceBasedPlan(
			job.Plan,
			resources,
			inputs), nil
	} else {
		populateLocations(&job.Plan)

		plan := factory.constructPlanHookBasedPlan(
			job.Plan,
			resources,
			inputs)
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

	if plan.OnSuccess != nil && (plan.OnSuccess.Next == atc.Plan{}) {
		plan.OnSuccess.Next = factory.constructPlanHookBasedPlan(
			planSequence[1:],
			resources,
			inputs,
		)
		return plan
	} else {
		return atc.Plan{
			OnSuccess: &atc.OnSuccessPlan{
				Step: plan,
				Next: factory.constructPlanHookBasedPlan(
					planSequence[1:],
					resources,
					inputs,
				),
			},
		}
	}
}

func populateLocations(planSequence *atc.PlanSequence) {
	p := *planSequence
	stepCount := uint(1)

	for i := 0; i < len(p); i++ {
		plan := p[i]
		location := &atc.Location{
			ID:            stepCount,
			ParentID:      0,
			ParallelGroup: 0,
		}
		stepCount = stepCount + populatePlanLocations(&plan, location)
		p[i] = plan
	}
}

func populatePlanLocations(planConfig *atc.PlanConfig, location *atc.Location) uint {
	var stepCount uint
	var parentID uint

	parentID = location.ID
	switch {
	case planConfig.Put != "":
		planConfig.Location = location
		// offset by one for the dependent get that will be added
		stepCount = stepCount + 1

	case planConfig.Do != nil:
		children := *planConfig.Do
		parentID = location.ID + 1
		for i := 0; i < len(children); i++ {
			child := children[i]
			childLocation := &atc.Location{
				ID:            location.ID + stepCount + 1,
				ParentID:      location.ParentID,
				ParallelGroup: 0,
				Hook:          location.Hook,
			}

			stepCount = stepCount + populatePlanLocations(&child, childLocation)
			children[i] = child
		}

	case planConfig.Try != nil:
		childLocation := &atc.Location{
			ID:            location.ID + stepCount + 1,
			ParentID:      location.ParentID,
			ParallelGroup: 0,
			Hook:          location.Hook,
		}
		stepCount = stepCount + populatePlanLocations(planConfig.Try, childLocation)

	case planConfig.Aggregate != nil:
		parallelGroup := location.ID + 1
		stepCount += 1

		if location.ParallelGroup != 0 {
			location.ParentID = location.ParallelGroup
		}

		children := *planConfig.Aggregate
		for i := 0; i < len(children); i++ {
			child := children[i]
			childLocation := &atc.Location{
				ID:            location.ID + stepCount + 1,
				ParentID:      location.ParentID,
				ParallelGroup: parallelGroup,
			}

			if child.Aggregate == nil {
				childLocation.Hook = location.Hook
			}

			stepCount = stepCount + populatePlanLocations(&child, childLocation)
			children[i] = child
		}

		parentID = parallelGroup
	default:
		planConfig.Location = location
	}

	if planConfig.Failure != nil {
		child := planConfig.Failure
		childLocation := &atc.Location{
			ID:            location.ID + stepCount + 1,
			ParentID:      parentID,
			ParallelGroup: 0,
			Hook:          "failure",
		}
		stepCount = stepCount + populatePlanLocations(child, childLocation)
	}
	if planConfig.Success != nil {
		child := planConfig.Success
		childLocation := &atc.Location{
			ID:            location.ID + stepCount + 1,
			ParentID:      parentID,
			ParallelGroup: 0,
			Hook:          "success",
		}
		stepCount = stepCount + populatePlanLocations(child, childLocation)
	}
	if planConfig.Ensure != nil {
		child := planConfig.Ensure
		childLocation := &atc.Location{
			ID:            location.ID + stepCount + 1,
			ParentID:      parentID,
			ParallelGroup: 0,
			Hook:          "ensure",
		}
		stepCount = stepCount + populatePlanLocations(child, childLocation)
	}
	return stepCount + 1
}

func (factory *BuildFactory) constructPlanSequenceBasedPlan(
	planSequence atc.PlanSequence,
	resources atc.ResourceConfigs,
	inputs []db.BuildInput,
) atc.Plan {
	if len(planSequence) == 0 {
		return atc.Plan{}
	}

	// Walk each plan in the plan sequence to determine the locations

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
		if plan.Location == nil {
			plan.Location = planConfig.Location
		}

	case planConfig.Put != "":
		logicalName := planConfig.Put

		resourceName := planConfig.Resource
		if resourceName == "" {
			resourceName = logicalName
		}

		resource, _ := resources.Lookup(resourceName)

		putPlan := &atc.PutPlan{
			Type:     resource.Type,
			Name:     logicalName,
			Pipeline: factory.PipelineName,
			Resource: resourceName,
			Source:   resource.Source,
			Params:   planConfig.Params,
			Tags:     planConfig.Tags,
		}

		dependentGetPlan := &atc.DependentGetPlan{
			Type:     resource.Type,
			Name:     logicalName,
			Pipeline: factory.PipelineName,
			Resource: resourceName,
			Params:   planConfig.GetParams,
			Tags:     planConfig.Tags,
		}

		stepLocation := &atc.Location{}
		nextLocation := &atc.Location{}

		if planConfig.Location != nil {
			stepLocation.ID = planConfig.Location.ID
			stepLocation.Hook = planConfig.Location.Hook

			if planConfig.Location.ParallelGroup != 0 {
				stepLocation.ParallelGroup = planConfig.Location.ParallelGroup
			} else {
				stepLocation.ParentID = planConfig.Location.ParentID
			}

			nextLocation.ID = stepLocation.ID + 1
			nextLocation.ParentID = stepLocation.ID
		}

		plan = atc.Plan{
			// Location: planConfig.Location,
			OnSuccess: &atc.OnSuccessPlan{
				Step: atc.Plan{
					Location: stepLocation,
					Put:      putPlan,
				},
				Next: atc.Plan{
					Location:     nextLocation,
					DependentGet: dependentGetPlan,
				},
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
			Location: planConfig.Location,
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
			Location: planConfig.Location,
			Task: &atc.TaskPlan{
				Name:       planConfig.Task,
				Privileged: planConfig.Privileged,
				Config:     planConfig.TaskConfig,
				ConfigPath: planConfig.TaskConfigPath,
				Tags:       planConfig.Tags,
			},
		}

	case planConfig.Try != nil:
		nextStep := factory.constructPlanFromConfig(
			*planConfig.Try,
			resources,
			inputs,
			hasHooks)

		plan = atc.Plan{
			Location: planConfig.Location,
			Try: &atc.TryPlan{
				Step: nextStep,
			},
		}

	case planConfig.Aggregate != nil:
		aggregate := atc.AggregatePlan{}

		for _, planConfig := range *planConfig.Aggregate {
			nextStep := factory.constructPlanFromConfig(
				planConfig,
				resources,
				inputs,
				hasHooks)

			aggregate = append(aggregate, nextStep)
		}

		plan = atc.Plan{
			Location:  planConfig.Location,
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

	constructionParams := factory.ensureIfPresent(factory.successIfPresent(factory.failureIfPresent(
		constructionParams{
			plan:       plan,
			planConfig: planConfig,
			resources:  resources,
			inputs:     inputs,
			hasHooks:   hasHooks,
		})),
	)

	return constructionParams.plan
}

type constructionParams struct {
	plan       atc.Plan
	planConfig atc.PlanConfig
	resources  atc.ResourceConfigs
	inputs     []db.BuildInput
	hasHooks   bool
}

func (factory *BuildFactory) successIfPresent(constructionParams constructionParams) constructionParams {
	if constructionParams.planConfig.Success != nil {

		nextPlan := factory.constructPlanFromConfig(
			*constructionParams.planConfig.Success,
			constructionParams.resources,
			constructionParams.inputs,
			constructionParams.hasHooks)

		constructionParams.plan = atc.Plan{
			OnSuccess: &atc.OnSuccessPlan{
				Step: constructionParams.plan,
				Next: nextPlan,
			},
		}
	}
	return constructionParams
}

func (factory *BuildFactory) failureIfPresent(constructionParams constructionParams) constructionParams {
	if constructionParams.planConfig.Failure != nil {
		nextPlan := factory.constructPlanFromConfig(
			*constructionParams.planConfig.Failure,
			constructionParams.resources,
			constructionParams.inputs,
			constructionParams.hasHooks)

		constructionParams.plan = atc.Plan{
			OnFailure: &atc.OnFailurePlan{
				Step: constructionParams.plan,
				Next: nextPlan,
			},
		}
	}

	return constructionParams
}

func (factory *BuildFactory) ensureIfPresent(constructionParams constructionParams) constructionParams {
	if constructionParams.planConfig.Ensure != nil {
		nextPlan := factory.constructPlanFromConfig(
			*constructionParams.planConfig.Ensure,
			constructionParams.resources,
			constructionParams.inputs,
			constructionParams.hasHooks)

		constructionParams.plan = atc.Plan{
			Ensure: &atc.EnsurePlan{
				Step: constructionParams.plan,
				Next: nextPlan,
			},
		}
	}
	return constructionParams
}
