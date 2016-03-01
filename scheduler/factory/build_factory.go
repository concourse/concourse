package factory

import (
	"errors"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

const defaultTaskName = "build"

var ErrResourceNotFound = errors.New("resource not found")

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Create(atc.JobConfig, atc.ResourceConfigs, atc.ResourceTypes, []db.BuildInput) (atc.Plan, error)
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
	resourceTypes atc.ResourceTypes,
	inputs []db.BuildInput,
) (atc.Plan, error) {
	planSequence := job.Plan

	if len(planSequence) == 1 {
		return factory.constructPlanFromConfig(
			planSequence[0],
			resources,
			resourceTypes,
			inputs,
		)
	}

	return factory.do(planSequence, resources, resourceTypes, inputs)
}

func (factory *buildFactory) do(
	planSequence atc.PlanSequence,
	resources atc.ResourceConfigs,
	resourceTypes atc.ResourceTypes,
	inputs []db.BuildInput,
) (atc.Plan, error) {
	do := atc.DoPlan{}

	var err error
	for _, planConfig := range planSequence {
		nextStep, err := factory.constructPlanFromConfig(
			planConfig,
			resources,
			resourceTypes,
			inputs,
		)
		if err != nil {
			return atc.Plan{}, err
		}

		do = append(do, nextStep)
	}

	return factory.planFactory.NewPlan(do), err
}

func (factory *buildFactory) constructPlanFromConfig(
	planConfig atc.PlanConfig,
	resources atc.ResourceConfigs,
	resourceTypes atc.ResourceTypes,
	inputs []db.BuildInput,
) (atc.Plan, error) {
	var plan atc.Plan
	var err error

	if planConfig.Attempts == 0 {
		plan, err = factory.constructUnhookedPlan(planConfig, resources, resourceTypes, inputs)
		if err != nil {
			return atc.Plan{}, err
		}
	} else {
		retryStep := make(atc.RetryPlan, planConfig.Attempts)

		for i := 0; i < planConfig.Attempts; i++ {
			attempt, err := factory.constructUnhookedPlan(planConfig, resources, resourceTypes, inputs)
			if err != nil {
				return atc.Plan{}, err
			}

			retryStep[i] = attempt
		}

		plan = factory.planFactory.NewPlan(retryStep)
	}

	constructionParams, err := factory.failureIfPresent(
		constructionParams{
			plan:          plan,
			planConfig:    planConfig,
			resources:     resources,
			resourceTypes: resourceTypes,
			inputs:        inputs,
		})
	if err != nil {
		return atc.Plan{}, err
	}

	constructionParams, err = factory.successIfPresent(constructionParams)
	if err != nil {
		return atc.Plan{}, err
	}

	constructionParams, err = factory.ensureIfPresent(constructionParams)
	if err != nil {
		return atc.Plan{}, err
	}

	return constructionParams.plan, nil
}

func (factory *buildFactory) constructUnhookedPlan(
	planConfig atc.PlanConfig,
	resources atc.ResourceConfigs,
	resourceTypes atc.ResourceTypes,
	inputs []db.BuildInput,
) (atc.Plan, error) {
	var plan atc.Plan
	var err error

	switch {
	case planConfig.Do != nil:
		plan, err = factory.do(
			*planConfig.Do,
			resources,
			resourceTypes,
			inputs,
		)
		if err != nil {
			return atc.Plan{}, err
		}

	case planConfig.Put != "":
		logicalName := planConfig.Put

		resourceName := planConfig.Resource
		if resourceName == "" {
			resourceName = logicalName
		}

		resource, found := resources.Lookup(resourceName)
		if !found {
			return atc.Plan{}, ErrResourceNotFound
		}

		putPlan := atc.PutPlan{
			Type:          resource.Type,
			Name:          logicalName,
			Pipeline:      factory.PipelineName,
			Resource:      resourceName,
			Source:        resource.Source,
			Params:        planConfig.Params,
			Tags:          planConfig.Tags,
			ResourceTypes: resourceTypes,
		}

		dependentGetPlan := atc.DependentGetPlan{
			Type:          resource.Type,
			Name:          logicalName,
			Pipeline:      factory.PipelineName,
			Resource:      resourceName,
			Params:        planConfig.GetParams,
			Tags:          planConfig.Tags,
			Source:        resource.Source,
			ResourceTypes: resourceTypes,
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

		resource, found := resources.Lookup(resourceName)
		if !found {
			return atc.Plan{}, ErrResourceNotFound
		}

		name := planConfig.Get
		var version db.Version
		for _, input := range inputs {
			if input.Name == name {
				version = input.Version
				break
			}
		}

		plan = factory.planFactory.NewPlan(atc.GetPlan{
			Type:          resource.Type,
			Name:          name,
			Pipeline:      factory.PipelineName,
			Resource:      resourceName,
			Source:        resource.Source,
			Params:        planConfig.Params,
			Version:       atc.Version(version),
			Tags:          planConfig.Tags,
			ResourceTypes: resourceTypes,
		})

	case planConfig.Task != "":
		plan = factory.planFactory.NewPlan(atc.TaskPlan{
			Name:          planConfig.Task,
			Pipeline:      factory.PipelineName,
			Privileged:    planConfig.Privileged,
			Config:        planConfig.TaskConfig,
			ConfigPath:    planConfig.TaskConfigPath,
			Tags:          planConfig.Tags,
			ResourceTypes: resourceTypes,
			Params:        planConfig.Params,
		})
	case planConfig.Try != nil:
		nextStep, err := factory.constructPlanFromConfig(
			*planConfig.Try,
			resources,
			resourceTypes,
			inputs,
		)
		if err != nil {
			return atc.Plan{}, err
		}

		plan = factory.planFactory.NewPlan(atc.TryPlan{
			Step: nextStep,
		})

	case planConfig.Aggregate != nil:
		aggregate := atc.AggregatePlan{}

		for _, planConfig := range *planConfig.Aggregate {
			nextStep, err := factory.constructPlanFromConfig(
				planConfig,
				resources,
				resourceTypes,
				inputs,
			)
			if err != nil {
				return atc.Plan{}, err
			}

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

	return plan, nil
}

type constructionParams struct {
	plan          atc.Plan
	planConfig    atc.PlanConfig
	resources     atc.ResourceConfigs
	resourceTypes atc.ResourceTypes
	inputs        []db.BuildInput
}

func (factory *buildFactory) successIfPresent(cp constructionParams) (constructionParams, error) {
	if cp.planConfig.Success != nil {

		nextPlan, err := factory.constructPlanFromConfig(
			*cp.planConfig.Success,
			cp.resources,
			cp.resourceTypes,
			cp.inputs,
		)
		if err != nil {
			return constructionParams{}, err
		}

		cp.plan = factory.planFactory.NewPlan(atc.OnSuccessPlan{
			Step: cp.plan,
			Next: nextPlan,
		})
	}
	return cp, nil
}

func (factory *buildFactory) failureIfPresent(cp constructionParams) (constructionParams, error) {
	if cp.planConfig.Failure != nil {
		nextPlan, err := factory.constructPlanFromConfig(
			*cp.planConfig.Failure,
			cp.resources,
			cp.resourceTypes,
			cp.inputs,
		)
		if err != nil {
			return constructionParams{}, err
		}

		cp.plan = factory.planFactory.NewPlan(atc.OnFailurePlan{
			Step: cp.plan,
			Next: nextPlan,
		})
	}

	return cp, nil
}

func (factory *buildFactory) ensureIfPresent(cp constructionParams) (constructionParams, error) {
	if cp.planConfig.Ensure != nil {
		nextPlan, err := factory.constructPlanFromConfig(
			*cp.planConfig.Ensure,
			cp.resources,
			cp.resourceTypes,
			cp.inputs,
		)
		if err != nil {
			return constructionParams{}, err
		}

		cp.plan = factory.planFactory.NewPlan(atc.EnsurePlan{
			Step: cp.plan,
			Next: nextPlan,
		})
	}
	return cp, nil
}
