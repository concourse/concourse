package factory

import (
	"errors"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var ErrResourceNotFound = errors.New("resource not found")

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Create(atc.JobConfig, atc.ResourceConfigs, atc.VersionedResourceTypes, []db.BuildInput) (atc.Plan, error)
}

type buildFactory struct {
	PipelineID  int
	planFactory atc.PlanFactory
}

func NewBuildFactory(pipelineID int, planFactory atc.PlanFactory) BuildFactory {
	return &buildFactory{
		PipelineID:  pipelineID,
		planFactory: planFactory,
	}
}

func (factory *buildFactory) Create(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	resourceTypes atc.VersionedResourceTypes,
	inputs []db.BuildInput,
) (atc.Plan, error) {
	plan, err := factory.constructPlanFromJob(job, resources, resourceTypes, inputs)
	if err != nil {
		return atc.Plan{}, err
	}

	return factory.applyHooks(constructionParams{
		plan:          plan,
		hooks:         job.Hooks(),
		resources:     resources,
		resourceTypes: resourceTypes,
		inputs:        inputs,
	})
}

func (factory *buildFactory) constructPlanFromJob(
	job atc.JobConfig,
	resources atc.ResourceConfigs,
	resourceTypes atc.VersionedResourceTypes,
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
	resourceTypes atc.VersionedResourceTypes,
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
	resourceTypes atc.VersionedResourceTypes,
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

	return factory.applyHooks(constructionParams{
		plan:          plan,
		hooks:         planConfig.Hooks(),
		resources:     resources,
		resourceTypes: resourceTypes,
		inputs:        inputs,
	})
}

func (factory *buildFactory) constructUnhookedPlan(
	planConfig atc.PlanConfig,
	resources atc.ResourceConfigs,
	resourceTypes atc.VersionedResourceTypes,
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

		putPlan := factory.planFactory.NewPlan(atc.PutPlan{
			Type:     resource.Type,
			Name:     logicalName,
			Resource: resourceName,
			Source:   resource.Source,
			Params:   planConfig.Params,
			Tags:     planConfig.Tags,

			VersionedResourceTypes: resourceTypes,
		})

		dependentGetPlan := factory.planFactory.NewPlan(atc.GetPlan{
			Type:        resource.Type,
			Name:        logicalName,
			Resource:    resourceName,
			VersionFrom: &putPlan.ID,

			Params: planConfig.GetParams,
			Tags:   planConfig.Tags,
			Source: resource.Source,

			VersionedResourceTypes: resourceTypes,
		})

		plan = factory.planFactory.NewPlan(atc.OnSuccessPlan{
			Step: putPlan,
			Next: dependentGetPlan,
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
		var version atc.Version
		for _, input := range inputs {
			if input.Name == name {
				version = atc.Version(input.Version)
				break
			}
		}

		plan = factory.planFactory.NewPlan(atc.GetPlan{
			Type:     resource.Type,
			Name:     name,
			Resource: resourceName,
			Source:   resource.Source,
			Params:   planConfig.Params,
			Version:  &version,
			Tags:     planConfig.Tags,

			VersionedResourceTypes: resourceTypes,
		})

	case planConfig.Task != "":
		plan = factory.planFactory.NewPlan(atc.TaskPlan{
			Name:              planConfig.Task,
			Privileged:        planConfig.Privileged,
			Config:            planConfig.TaskConfig,
			ConfigPath:        planConfig.TaskConfigPath,
			Tags:              planConfig.Tags,
			Params:            planConfig.Params,
			InputMapping:      planConfig.InputMapping,
			OutputMapping:     planConfig.OutputMapping,
			ImageArtifactName: planConfig.ImageArtifactName,

			VersionedResourceTypes: resourceTypes,
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
	hooks         atc.Hooks
	resources     atc.ResourceConfigs
	resourceTypes atc.VersionedResourceTypes
	inputs        []db.BuildInput
}

func (factory *buildFactory) applyHooks(cp constructionParams) (atc.Plan, error) {
	var err error

	cp, err = factory.abortIfPresent(cp)
	if err != nil {
		return atc.Plan{}, err
	}

	cp, err = factory.failureIfPresent(cp)
	if err != nil {
		return atc.Plan{}, err
	}

	cp, err = factory.successIfPresent(cp)
	if err != nil {
		return atc.Plan{}, err
	}

	cp, err = factory.ensureIfPresent(cp)
	if err != nil {
		return atc.Plan{}, err
	}

	return cp.plan, nil
}

func (factory *buildFactory) successIfPresent(cp constructionParams) (constructionParams, error) {
	if cp.hooks.Success != nil {

		nextPlan, err := factory.constructPlanFromConfig(
			*cp.hooks.Success,
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
	if cp.hooks.Failure != nil {
		nextPlan, err := factory.constructPlanFromConfig(
			*cp.hooks.Failure,
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
	if cp.hooks.Ensure != nil {
		nextPlan, err := factory.constructPlanFromConfig(
			*cp.hooks.Ensure,
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

func (factory *buildFactory) abortIfPresent(cp constructionParams) (constructionParams, error) {
	if cp.hooks.Abort != nil {
		nextPlan, err := factory.constructPlanFromConfig(
			*cp.hooks.Abort,
			cp.resources,
			cp.resourceTypes,
			cp.inputs,
		)
		if err != nil {
			return constructionParams{}, err
		}

		cp.plan = factory.planFactory.NewPlan(atc.OnAbortPlan{
			Step: cp.plan,
			Next: nextPlan,
		})
	}

	return cp, nil
}
