package builds

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

type Planner struct {
	planFactory atc.PlanFactory
}

func NewPlanner(planFactory atc.PlanFactory) Planner {
	return Planner{
		planFactory: planFactory,
	}
}

func (planner Planner) Create(
	planConfig atc.PlanConfig,
	resources db.SchedulerResources,
	resourceTypes atc.VersionedResourceTypes,
	inputs []db.BuildInput,
) (atc.Plan, error) {
	var plan atc.Plan
	var err error

	if planConfig.Attempts == 0 {
		plan, err = planner.basePlan(planConfig, resources, resourceTypes, inputs)
		if err != nil {
			return atc.Plan{}, err
		}
	} else {
		retryStep := make(atc.RetryPlan, planConfig.Attempts)

		for i := 0; i < planConfig.Attempts; i++ {
			attempt, err := planner.basePlan(planConfig, resources, resourceTypes, inputs)
			if err != nil {
				return atc.Plan{}, err
			}

			retryStep[i] = attempt
		}

		plan = planner.planFactory.NewPlan(retryStep)
	}

	if planConfig.Abort != nil {
		hookPlan, err := planner.Create(
			*planConfig.Abort,
			resources,
			resourceTypes,
			inputs,
		)
		if err != nil {
			return atc.Plan{}, err
		}

		plan = planner.planFactory.NewPlan(atc.OnAbortPlan{
			Step: plan,
			Next: hookPlan,
		})
	}

	if planConfig.Error != nil {
		hookPlan, err := planner.Create(
			*planConfig.Error,
			resources,
			resourceTypes,
			inputs,
		)
		if err != nil {
			return atc.Plan{}, err
		}

		plan = planner.planFactory.NewPlan(atc.OnErrorPlan{
			Step: plan,
			Next: hookPlan,
		})
	}

	if planConfig.Failure != nil {
		hookPlan, err := planner.Create(
			*planConfig.Failure,
			resources,
			resourceTypes,
			inputs,
		)
		if err != nil {
			return atc.Plan{}, err
		}

		plan = planner.planFactory.NewPlan(atc.OnFailurePlan{
			Step: plan,
			Next: hookPlan,
		})
	}

	if planConfig.Success != nil {
		hookPlan, err := planner.Create(
			*planConfig.Success,
			resources,
			resourceTypes,
			inputs,
		)
		if err != nil {
			return atc.Plan{}, err
		}

		plan = planner.planFactory.NewPlan(atc.OnSuccessPlan{
			Step: plan,
			Next: hookPlan,
		})
	}

	if planConfig.Ensure != nil {
		hookPlan, err := planner.Create(
			*planConfig.Ensure,
			resources,
			resourceTypes,
			inputs,
		)
		if err != nil {
			return atc.Plan{}, err
		}

		plan = planner.planFactory.NewPlan(atc.EnsurePlan{
			Step: plan,
			Next: hookPlan,
		})
	}

	return plan, nil
}

func (factory Planner) basePlan(
	planConfig atc.PlanConfig,
	resources db.SchedulerResources,
	resourceTypes atc.VersionedResourceTypes,
	inputs []db.BuildInput,
) (atc.Plan, error) {
	var plan atc.Plan

	switch {
	case planConfig.Do != nil:
		do := atc.DoPlan{}

		for _, planConfig := range *planConfig.Do {
			nextStep, err := factory.Create(
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

		plan = factory.planFactory.NewPlan(do)

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

		if version == nil {
			return atc.Plan{}, VersionNotFoundError{name}
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
			ConfigPath:        planConfig.File,
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
			File:     planConfig.File,
			Team:     planConfig.Team,
			Vars:     planConfig.Vars,
			VarFiles: planConfig.VarFiles,
		})

	case planConfig.LoadVar != "":
		name := planConfig.LoadVar
		plan = factory.planFactory.NewPlan(atc.LoadVarPlan{
			Name:   name,
			File:   planConfig.File,
			Format: planConfig.Format,
			Reveal: planConfig.Reveal,
		})

	case planConfig.Try != nil:
		nextStep, err := factory.Create(
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
			nextStep, err := factory.Create(
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
			step, err := factory.Create(
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
