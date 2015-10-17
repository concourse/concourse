package factory

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

const defaultTaskName = "build"

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Create(atc.JobConfig, atc.ResourceConfigs, []db.BuildInput) atc.Plan
}

type buildFactory struct {
	PipelineName      string
	LocationPopulator LocationPopulator
}

func NewBuildFactory(pipelineName string, lp LocationPopulator) BuildFactory {
	return &buildFactory{
		PipelineName:      pipelineName,
		LocationPopulator: lp,
	}
}

func (factory *buildFactory) Create(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	inputs []db.BuildInput,
) atc.Plan {
	factory.LocationPopulator.PopulateLocations(&job.Plan)

	return factory.constructPlanFromSequence(
		job.Plan,
		resources,
		inputs,
	)
}

func (factory *buildFactory) constructPlanFromSequence(
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
	)

	if len(planSequence) == 1 {
		return plan
	}

	if plan.OnSuccess != nil && (plan.OnSuccess.Next == atc.Plan{}) {
		plan.OnSuccess.Next = factory.constructPlanFromSequence(
			planSequence[1:],
			resources,
			inputs,
		)
		return plan
	} else {
		return atc.Plan{
			OnSuccess: &atc.OnSuccessPlan{
				Step: plan,
				Next: factory.constructPlanFromSequence(
					planSequence[1:],
					resources,
					inputs,
				),
			},
		}
	}
}

func (factory *buildFactory) constructPlanFromConfig(
	planConfig atc.PlanConfig,
	resources atc.ResourceConfigs,
	inputs []db.BuildInput,
) atc.Plan {
	var plan atc.Plan

	switch {
	case planConfig.Do != nil:
		plan = factory.constructPlanFromSequence(
			*planConfig.Do,
			resources,
			inputs,
		)

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
				Pipeline:   factory.PipelineName,
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
		)

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
			)

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
		})),
	)

	return constructionParams.plan
}

type constructionParams struct {
	plan       atc.Plan
	planConfig atc.PlanConfig
	resources  atc.ResourceConfigs
	inputs     []db.BuildInput
}

func (factory *buildFactory) successIfPresent(constructionParams constructionParams) constructionParams {
	if constructionParams.planConfig.Success != nil {

		nextPlan := factory.constructPlanFromConfig(
			*constructionParams.planConfig.Success,
			constructionParams.resources,
			constructionParams.inputs,
		)

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
		)

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
		)

		constructionParams.plan = atc.Plan{
			Ensure: &atc.EnsurePlan{
				Step: constructionParams.plan,
				Next: nextPlan,
			},
		}
	}
	return constructionParams
}
