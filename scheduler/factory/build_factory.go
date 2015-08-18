package factory

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

const defaultTaskName = "build"

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Create(atc.JobConfig,
		atc.ResourceConfigs,
		[]db.BuildInput,
	) (atc.Plan, error)
}

type buildFactory struct {
	PipelineName      string
	LocationPopulator LocationPopulator
}

func NewBuildFactory(pipelineName string, lp LocationPopulator) BuildFactory {
	return &buildFactory{
		PipelineName: pipelineName,
		// LocationPopulator: NewLocationPopulator(),
		LocationPopulator: lp,
	}
}

func (factory *buildFactory) Create(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	inputs []db.BuildInput,
) (atc.Plan, error) {

	factory.LocationPopulator.PopulateLocations(&job.Plan)

	plan := factory.constructPlanHookBasedPlan(
		job.Plan,
		resources,
		inputs)
	return plan, nil
}

func (factory *buildFactory) doesAnyStepMatch(planSequence atc.PlanSequence, predicate func(step atc.PlanConfig) bool) bool {
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

func (factory *buildFactory) constructPlanHookBasedPlan(
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

func (factory *buildFactory) constructPlanSequenceBasedPlan(
	planSequence atc.PlanSequence,
	resources atc.ResourceConfigs,
	inputs []db.BuildInput,
) atc.Plan {
	if len(planSequence) == 0 {
		return atc.Plan{}
	}

	var plan atc.Plan
	// Walk each plan in the plan sequence to determine the locations
	for i := 0; i < len(planSequence); i++ {
		// plan preceding the current one in the sequence
		plan = factory.constructPlanFromConfig(
			planSequence[i],
			resources,
			inputs,
			false,
		)

	}

	return plan
}

func (factory *buildFactory) constructPlanFromConfig(
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
			Source:   resource.Source,
		}

		stepLocation := &atc.Location{}
		nextLocation := &atc.Location{}

		if planConfig.Location != nil {
			stepLocation.ID = planConfig.Location.ID
			stepLocation.Hook = planConfig.Location.Hook
			stepLocation.SerialGroup = planConfig.Location.SerialGroup

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

func (factory *buildFactory) successIfPresent(constructionParams constructionParams) constructionParams {
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

func (factory *buildFactory) failureIfPresent(constructionParams constructionParams) constructionParams {
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

func (factory *buildFactory) ensureIfPresent(constructionParams constructionParams) constructionParams {
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
