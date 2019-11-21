package factory

import (
	"errors"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

var ErrResourceNotFound = errors.New("resource not found")

//go:generate counterfeiter . BuildFactory

type BuildFactory interface {
	Create(atc.JobConfig, atc.ResourceConfigs, atc.VersionedResourceTypes, []db.BuildInput) (atc.Plan, error)
}

type buildFactory struct {
	planFactory atc.PlanFactory
}

func NewBuildFactory(planFactory atc.PlanFactory) BuildFactory {
	return &buildFactory{
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

	return factory.applyHooks(job, constructionParams{
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
			job,
			planSequence[0],
			resources,
			resourceTypes,
			inputs,
		)
	}

	return factory.do(job, planSequence, resources, resourceTypes, inputs)
}

func (factory *buildFactory) do(
	job atc.JobConfig,
	planSequence atc.PlanSequence,
	resources atc.ResourceConfigs,
	resourceTypes atc.VersionedResourceTypes,
	inputs []db.BuildInput,
) (atc.Plan, error) {
	do := atc.DoPlan{}

	var err error
	for _, planConfig := range planSequence {
		nextStep, err := factory.constructPlanFromConfig(
			job,
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
	job atc.JobConfig,
	planConfig atc.PlanConfig,
	resources atc.ResourceConfigs,
	resourceTypes atc.VersionedResourceTypes,
	inputs []db.BuildInput,
) (atc.Plan, error) {
	var plan atc.Plan
	var err error

	if planConfig.Attempts == 0 {
		plan, err = factory.constructUnhookedPlan(job, planConfig, resources, resourceTypes, inputs)
		if err != nil {
			return atc.Plan{}, err
		}
	} else {
		retryStep := make(atc.RetryPlan, planConfig.Attempts)

		for i := 0; i < planConfig.Attempts; i++ {
			attempt, err := factory.constructUnhookedPlan(job, planConfig, resources, resourceTypes, inputs)
			if err != nil {
				return atc.Plan{}, err
			}

			retryStep[i] = attempt
		}

		plan = factory.planFactory.NewPlan(retryStep)
	}

	return factory.applyHooks(job, constructionParams{
		plan:          plan,
		hooks:         planConfig.Hooks(),
		resources:     resources,
		resourceTypes: resourceTypes,
		inputs:        inputs,
	})
}

func (factory *buildFactory) constructUnhookedPlan(
	job atc.JobConfig,
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
			job,
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

		atcPutPlan := atc.PutPlan{
			Type:     resource.Type,
			Name:     logicalName,
			Resource: resourceName,
			Source:   resource.Source,
			Params:   planConfig.Params,
			Tags:     planConfig.Tags,
			Inputs:   planConfig.Inputs,

			VersionedResourceTypes: resourceTypes,
		}

		putPlan := factory.planFactory.NewPlan(atcPutPlan)

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
			ConfigPath:        planConfig.ConfigPath,
			Vars:              planConfig.Vars,
			Tags:              planConfig.Tags,
			Params:            planConfig.Params,
			InputMapping:      planConfig.InputMapping,
			OutputMapping:     planConfig.OutputMapping,
			ImageArtifactName: planConfig.ImageArtifactName,

			VersionedResourceTypes: resourceTypes,
		})

	case planConfig.SetPipeline != "":
		name := planConfig.SetPipeline
		plan = factory.planFactory.NewPlan(atc.SetPipelinePlan{
			Name:     name,
			File:     planConfig.ConfigPath,
			Vars:     planConfig.Vars,
			VarFiles: planConfig.VarFiles,
		})

	case planConfig.Try != nil:
		nextStep, err := factory.constructPlanFromConfig(
			job,
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
				job,
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

	case planConfig.InParallel != nil:
		var steps []atc.Plan

		for _, planConfig := range planConfig.InParallel.Steps {
			step, err := factory.constructPlanFromConfig(
				job,
				planConfig,
				resources,
				resourceTypes,
				inputs,
			)
			if err != nil {
				return atc.Plan{}, err
			}

			steps = append(steps, step)
		}

		plan = factory.planFactory.NewPlan(atc.InParallelPlan{
			Steps:    steps,
			Limit:    planConfig.InParallel.Limit,
			FailFast: planConfig.InParallel.FailFast,
		})
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

func (factory *buildFactory) applyHooks(job atc.JobConfig, cp constructionParams) (atc.Plan, error) {
	var err error

	cp, err = factory.abortIfPresent(job, cp)
	if err != nil {
		return atc.Plan{}, err
	}

	cp, err = factory.errorIfPresent(job, cp)
	if err != nil {
		return atc.Plan{}, err
	}

	cp, err = factory.failureIfPresent(job, cp)
	if err != nil {
		return atc.Plan{}, err
	}

	cp, err = factory.successIfPresent(job, cp)
	if err != nil {
		return atc.Plan{}, err
	}

	cp, err = factory.ensureIfPresent(job, cp)
	if err != nil {
		return atc.Plan{}, err
	}

	return cp.plan, nil
}

func (factory *buildFactory) successIfPresent(job atc.JobConfig, cp constructionParams) (constructionParams, error) {
	if cp.hooks.Success != nil {

		nextPlan, err := factory.constructPlanFromConfig(
			job,
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

func (factory *buildFactory) failureIfPresent(job atc.JobConfig, cp constructionParams) (constructionParams, error) {
	if cp.hooks.Failure != nil {
		nextPlan, err := factory.constructPlanFromConfig(
			job,
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

func (factory *buildFactory) ensureIfPresent(job atc.JobConfig, cp constructionParams) (constructionParams, error) {
	if cp.hooks.Ensure != nil {
		nextPlan, err := factory.constructPlanFromConfig(
			job,
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

func (factory *buildFactory) abortIfPresent(job atc.JobConfig, cp constructionParams) (constructionParams, error) {
	if cp.hooks.Abort != nil {
		nextPlan, err := factory.constructPlanFromConfig(
			job,
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

func (factory *buildFactory) errorIfPresent(job atc.JobConfig, cp constructionParams) (constructionParams, error) {
	if cp.hooks.Error != nil {
		nextPlan, err := factory.constructPlanFromConfig(
			job,
			*cp.hooks.Error,
			cp.resources,
			cp.resourceTypes,
			cp.inputs,
		)
		if err != nil {
			return constructionParams{}, err
		}

		cp.plan = factory.planFactory.NewPlan(atc.OnErrorPlan{
			Step: cp.plan,
			Next: nextPlan,
		})
	}

	return cp, nil
}
