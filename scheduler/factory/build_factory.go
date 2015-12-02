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
	PipelineName string
	planFactory  atc.PlanFactory
}

func NewBuildFactory(pipelineName string, planFactory atc.PlanFactory) BuildFactory {
	return &buildFactory{
		PipelineName: pipelineName,
		planFactory:  planFactory,
	}
}

func (factory *buildFactory) Create(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	inputs []db.BuildInput,
) atc.Plan {
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
		return factory.planFactory.NewPlan(atc.OnSuccessPlan{
			Step: plan,
			Next: factory.constructPlanFromSequence(
				planSequence[1:],
				resources,
				inputs,
			),
		})
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

	case planConfig.Put != "":
		logicalName := planConfig.Put

		resourceName := planConfig.Resource
		if resourceName == "" {
			resourceName = logicalName
		}

		resource, _ := resources.Lookup(resourceName)

		putPlan := atc.PutPlan{
			Type:     resource.Type,
			Name:     logicalName,
			Pipeline: factory.PipelineName,
			Resource: resourceName,
			Source:   resource.Source,
			Params:   planConfig.Params,
			Tags:     planConfig.Tags,
		}

		dependentGetPlan := atc.DependentGetPlan{
			Type:     resource.Type,
			Name:     logicalName,
			Pipeline: factory.PipelineName,
			Resource: resourceName,
			Params:   planConfig.GetParams,
			Tags:     planConfig.Tags,
			Source:   resource.Source,
		}

		plan = factory.planFactory.NewPlan(atc.OnSuccessPlan{
			Step: factory.planFactory.NewPlan(putPlan),
			Next: factory.planFactory.NewPlan(dependentGetPlan),
		})

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

		plan = factory.planFactory.NewPlan(atc.GetPlan{
			Type:     resource.Type,
			Name:     name,
			Pipeline: factory.PipelineName,
			Resource: resourceName,
			Source:   resource.Source,
			Params:   planConfig.Params,
			Version:  atc.Version(version),
			Tags:     planConfig.Tags,
		})

	case planConfig.Task != "":
		plan = factory.planFactory.NewPlan(atc.TaskPlan{
			Name:       planConfig.Task,
			Pipeline:   factory.PipelineName,
			Privileged: planConfig.Privileged,
			Config:     planConfig.TaskConfig,
			ConfigPath: planConfig.TaskConfigPath,
			Tags:       planConfig.Tags,
		})

	case planConfig.Try != nil:
		nextStep := factory.constructPlanFromConfig(
			*planConfig.Try,
			resources,
			inputs,
		)

		plan = factory.planFactory.NewPlan(atc.TryPlan{
			Step: nextStep,
		})

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

		plan = factory.planFactory.NewPlan(aggregate)
	}

	if planConfig.Timeout != "" {
		plan = factory.planFactory.NewPlan(atc.TimeoutPlan{
			Duration: planConfig.Timeout,
			Step:     plan,
		})
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

		constructionParams.plan = factory.planFactory.NewPlan(atc.OnSuccessPlan{
			Step: constructionParams.plan,
			Next: nextPlan,
		})
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

		constructionParams.plan = factory.planFactory.NewPlan(atc.OnFailurePlan{
			Step: constructionParams.plan,
			Next: nextPlan,
		})
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

		constructionParams.plan = factory.planFactory.NewPlan(atc.EnsurePlan{
			Step: constructionParams.plan,
			Next: nextPlan,
		})
	}
	return constructionParams
}
