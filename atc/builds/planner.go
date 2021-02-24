package builds

import (
	"time"

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
	planConfig atc.StepConfig,
	resources db.SchedulerResources,
	resourceTypes atc.VersionedResourceTypes,
	inputs []db.BuildInput,
) (atc.Plan, error) {
	visitor := &planVisitor{
		planFactory: planner.planFactory,

		resources:     resources,
		resourceTypes: resourceTypes,
		inputs:        inputs,
	}

	err := planConfig.Visit(visitor)
	if err != nil {
		return atc.Plan{}, err
	}

	return visitor.plan, nil
}

type planVisitor struct {
	planFactory atc.PlanFactory

	resources     db.SchedulerResources
	resourceTypes atc.VersionedResourceTypes
	inputs        []db.BuildInput

	plan atc.Plan
}

func (visitor *planVisitor) VisitTask(step *atc.TaskStep) error {
	visitor.plan = visitor.planFactory.NewPlan(atc.TaskPlan{
		Name:                   step.Name,
		Privileged:             step.Privileged,
		Config:                 step.Config,
		ConfigPath:             step.ConfigPath,
		VersionedResourceTypes: visitor.resourceTypes,
		Vars:                   step.Vars,
		Tags:                   step.Tags,
		Params:                 step.Params,
		InputMapping:           step.InputMapping,
		OutputMapping:          step.OutputMapping,
		ImageArtifactName:      step.ImageArtifactName,
	})

	return nil
}

func (visitor *planVisitor) VisitGet(step *atc.GetStep) error {
	resourceName := step.Resource
	if resourceName == "" {
		resourceName = step.Name
	}

	resource, found := visitor.resources.Lookup(resourceName)
	if !found {
		return UnknownResourceError{resourceName}
	}

	var version atc.Version
	for _, input := range visitor.inputs {
		if input.Name == step.Name {
			version = atc.Version(input.Version)
			break
		}
	}

	if version == nil {
		return VersionNotProvidedError{step.Name}
	}

	resource.ApplySourceDefaults(visitor.resourceTypes)

	getPlan := atc.GetPlan{
		Name: step.Name,

		Type:     resource.Type,
		Resource: resourceName,
		Source:   resource.Source,
		Params:   step.Params,
		Version:  &version,
		Tags:     step.Tags,

		VersionedResourceTypes: visitor.resourceTypes,
	}

	var plan atc.PlanConfig
	var imageFetchPlan atc.Plan
	imageFetchPlan, getPlan.ImageSpecFrom, getPlan.BaseImageType = visitor.fetchImagePlan(resource.Type, visitor.resourceTypes, step.Tags)
	if getPlan.ImageSpecFrom != nil {
		plan = atc.OnSuccessPlan{
			Step: imageFetchPlan,
			Next: visitor.planFactory.NewPlan(getPlan),
		}
	} else {
		plan = getPlan
	}

	visitor.plan = visitor.planFactory.NewPlan(plan)
	return nil
}

func (visitor *planVisitor) VisitPut(step *atc.PutStep) error {
	logicalName := step.Name

	resourceName := step.Resource
	if resourceName == "" {
		resourceName = logicalName
	}

	resource, found := visitor.resources.Lookup(resourceName)
	if !found {
		return UnknownResourceError{resourceName}
	}

	resource.ApplySourceDefaults(visitor.resourceTypes)

	atcPutPlan := atc.PutPlan{
		Type:     resource.Type,
		Name:     logicalName,
		Resource: resourceName,
		Source:   resource.Source,
		Params:   step.Params,
		Tags:     step.Tags,
		Inputs:   step.Inputs,

		VersionedResourceTypes: visitor.resourceTypes,
	}

	var plan atc.Plan
	var imageFetchPlan atc.Plan
	imageFetchPlan, atcPutPlan.ImageSpecFrom, atcPutPlan.BaseImageType = visitor.fetchImagePlan(resource.Type, visitor.resourceTypes, step.Tags)

	putPlan := visitor.planFactory.NewPlan(atcPutPlan)

	if atcPutPlan.ImageSpecFrom != nil {
		plan = visitor.planFactory.NewPlan(atc.OnSuccessPlan{
			Step: imageFetchPlan,
			Next: putPlan,
		})
	} else {
		plan = putPlan
	}

	dependentGetPlan := visitor.planFactory.NewPlan(atc.GetPlan{
		Type:        resource.Type,
		Name:        logicalName,
		Resource:    resourceName,
		VersionFrom: &putPlan.ID,

		ImageSpecFrom: putPlan.Put.ImageSpecFrom,
		BaseImageType: putPlan.Put.BaseImageType,

		Params: step.GetParams,
		Tags:   step.Tags,
		Source: resource.Source,

		VersionedResourceTypes: visitor.resourceTypes,
	})

	visitor.plan = visitor.planFactory.NewPlan(atc.OnSuccessPlan{
		Step: plan,
		Next: dependentGetPlan,
	})

	return nil
}

func (visitor *planVisitor) VisitDo(step *atc.DoStep) error {
	do := atc.DoPlan{}

	for _, step := range step.Steps {
		err := step.Config.Visit(visitor)
		if err != nil {
			return err
		}

		do = append(do, visitor.plan)
	}

	visitor.plan = visitor.planFactory.NewPlan(do)

	return nil
}

func (visitor *planVisitor) VisitAggregate(step *atc.AggregateStep) error {
	do := atc.AggregatePlan{}

	for _, sub := range step.Steps {
		err := sub.Config.Visit(visitor)
		if err != nil {
			return err
		}

		do = append(do, visitor.plan)
	}

	visitor.plan = visitor.planFactory.NewPlan(do)

	return nil
}

func (visitor *planVisitor) VisitInParallel(step *atc.InParallelStep) error {
	var steps []atc.Plan

	for _, sub := range step.Config.Steps {
		err := sub.Config.Visit(visitor)
		if err != nil {
			return err
		}

		steps = append(steps, visitor.plan)
	}

	visitor.plan = visitor.planFactory.NewPlan(atc.InParallelPlan{
		Steps:    steps,
		Limit:    step.Config.Limit,
		FailFast: step.Config.FailFast,
	})

	return nil
}

func (visitor *planVisitor) VisitAcross(step *atc.AcrossStep) error {
	vars := make([]atc.AcrossVar, len(step.Vars))
	for i, v := range step.Vars {
		vars[i] = atc.AcrossVar{
			Var:         v.Var,
			Values:      v.Values,
			MaxInFlight: v.MaxInFlight,
		}
	}

	acrossPlan := atc.AcrossPlan{
		Vars:     vars,
		Steps:    []atc.VarScopedPlan{},
		FailFast: step.FailFast,
	}
	for _, vals := range cartesianProduct(step.Vars) {
		err := step.Step.Visit(visitor)
		if err != nil {
			return err
		}
		acrossPlan.Steps = append(acrossPlan.Steps, atc.VarScopedPlan{
			Step:   visitor.plan,
			Values: vals,
		})
	}

	visitor.plan = visitor.planFactory.NewPlan(acrossPlan)

	return nil
}

func cartesianProduct(vars []atc.AcrossVarConfig) [][]interface{} {
	if len(vars) == 0 {
		return make([][]interface{}, 1)
	}
	var product [][]interface{}
	subProduct := cartesianProduct(vars[:len(vars)-1])
	for _, vec := range subProduct {
		for _, val := range vars[len(vars)-1].Values {
			product = append(product, append(vec, val))
		}
	}
	return product
}

func (visitor *planVisitor) VisitSetPipeline(step *atc.SetPipelineStep) error {
	visitor.plan = visitor.planFactory.NewPlan(atc.SetPipelinePlan{
		Name:         step.Name,
		File:         step.File,
		Team:         step.Team,
		Vars:         step.Vars,
		VarFiles:     step.VarFiles,
		InstanceVars: step.InstanceVars,
	})

	return nil
}

func (visitor *planVisitor) VisitLoadVar(step *atc.LoadVarStep) error {
	visitor.plan = visitor.planFactory.NewPlan(atc.LoadVarPlan{
		Name:   step.Name,
		File:   step.File,
		Format: step.Format,
		Reveal: step.Reveal,
	})

	return nil
}

func (visitor *planVisitor) VisitTry(step *atc.TryStep) error {
	err := step.Step.Config.Visit(visitor)
	if err != nil {
		return err
	}

	visitor.plan = visitor.planFactory.NewPlan(atc.TryPlan{
		Step: visitor.plan,
	})

	return nil
}

func (visitor *planVisitor) VisitTimeout(step *atc.TimeoutStep) error {
	err := step.Step.Visit(visitor)
	if err != nil {
		return err
	}

	visitor.plan = visitor.planFactory.NewPlan(atc.TimeoutPlan{
		Duration: step.Duration,
		Step:     visitor.plan,
	})

	return nil
}

func (visitor *planVisitor) VisitRetry(step *atc.RetryStep) error {
	retryStep := make(atc.RetryPlan, step.Attempts)

	for i := 0; i < step.Attempts; i++ {
		err := step.Step.Visit(visitor)
		if err != nil {
			return err
		}

		retryStep[i] = visitor.plan
	}

	visitor.plan = visitor.planFactory.NewPlan(retryStep)

	return nil
}

func (visitor *planVisitor) VisitOnSuccess(step *atc.OnSuccessStep) error {
	plan := atc.OnSuccessPlan{}

	err := step.Step.Visit(visitor)
	if err != nil {
		return err
	}

	plan.Step = visitor.plan

	err = step.Hook.Config.Visit(visitor)
	if err != nil {
		return err
	}

	plan.Next = visitor.plan

	visitor.plan = visitor.planFactory.NewPlan(plan)

	return nil
}

func (visitor *planVisitor) VisitOnFailure(step *atc.OnFailureStep) error {
	plan := atc.OnFailurePlan{}

	err := step.Step.Visit(visitor)
	if err != nil {
		return err
	}

	plan.Step = visitor.plan

	err = step.Hook.Config.Visit(visitor)
	if err != nil {
		return err
	}

	plan.Next = visitor.plan

	visitor.plan = visitor.planFactory.NewPlan(plan)

	return nil
}

func (visitor *planVisitor) VisitOnAbort(step *atc.OnAbortStep) error {
	plan := atc.OnAbortPlan{}

	err := step.Step.Visit(visitor)
	if err != nil {
		return err
	}

	plan.Step = visitor.plan

	err = step.Hook.Config.Visit(visitor)
	if err != nil {
		return err
	}

	plan.Next = visitor.plan

	visitor.plan = visitor.planFactory.NewPlan(plan)

	return nil
}

func (visitor *planVisitor) VisitOnError(step *atc.OnErrorStep) error {
	plan := atc.OnErrorPlan{}

	err := step.Step.Visit(visitor)
	if err != nil {
		return err
	}

	plan.Step = visitor.plan

	err = step.Hook.Config.Visit(visitor)
	if err != nil {
		return err
	}

	plan.Next = visitor.plan

	visitor.plan = visitor.planFactory.NewPlan(plan)

	return nil
}
func (visitor *planVisitor) VisitEnsure(step *atc.EnsureStep) error {
	plan := atc.EnsurePlan{}

	err := step.Step.Visit(visitor)
	if err != nil {
		return err
	}

	plan.Step = visitor.plan

	err = step.Hook.Config.Visit(visitor)
	if err != nil {
		return err
	}

	plan.Next = visitor.plan

	visitor.plan = visitor.planFactory.NewPlan(plan)

	return nil
}

type CheckPlanner struct {
	planFactory atc.PlanFactory
}

func (c *CheckPlanner) Create(resource db.Resource, resourceTypes db.ResourceTypes, from atc.Version, sourceDefaults atc.Source, interval time.Duration) atc.Plan {
	versionedResourceTypes := resourceTypes.Deserialize()

	checkPlan := atc.CheckPlan{
		Name:    resource.Name(),
		Type:    resource.Type(),
		Source:  sourceDefaults.Merge(resource.Source()),
		Tags:    resource.Tags(),
		Timeout: resource.CheckTimeout(),

		FromVersion:            from,
		Interval:               interval.String(),
		VersionedResourceTypes: versionedResourceTypes,

		Resource: resource.Name(),
	}

	var plan atc.Plan
	var imageFetchPlan atc.Plan
	imageFetchPlan, checkPlan.ImageSpecFrom, checkPlan.BaseImageType = fetchImagePlan(c.planFactory, resource.Type(), checkPlan.VersionedResourceTypes, resource.Tags())
	if checkPlan.ImageSpecFrom != nil {
		plan = c.planFactory.NewPlan(atc.OnSuccessPlan{
			Step: imageFetchPlan,
			Next: c.planFactory.NewPlan(checkPlan),
		})
	} else {
		plan = c.planFactory.NewPlan(checkPlan)
	}

	return plan
}

func fetchImagePlan(planFactory atc.PlanFactory, resourceTypeName string, resourceTypes atc.VersionedResourceTypes, stepTags atc.Tags) (atc.Plan, *atc.PlanID, *string) {
	// Check if the resource type is a custom resource type
	parentResourceType, found := resourceTypes.Lookup(resourceTypeName)
	if found {
		trimmedResourceTypes := resourceTypes.Without(resourceTypeName)

		// If resource type is a custom type, recurse in order to resolve nested resource types
		subPlan, subPlanID, subBaseImageType := fetchImagePlan(planFactory, parentResourceType.Type, trimmedResourceTypes, stepTags)

		tags := parentResourceType.Tags
		if len(parentResourceType.Tags) == 0 {
			tags = stepTags
		}

		// Construct get plan for image
		var imageFetchPlan atc.Plan
		imageGetConfig := atc.GetPlan{
			Name:   parentResourceType.Name,
			Type:   parentResourceType.Type,
			Source: parentResourceType.Source,
			Params: parentResourceType.Params,

			ImageSpecFrom: subPlanID,
			BaseImageType: subBaseImageType,

			VersionedResourceTypes: trimmedResourceTypes,

			Tags: tags,
		}

		var imageGetPlan atc.Plan
		resourceTypeVersion := parentResourceType.Version
		if resourceTypeVersion == nil {
			// don't know the version, need to do a Check before the Get
			checkPlan := planFactory.NewPlan(atc.CheckPlan{
				Name:   parentResourceType.Name,
				Type:   parentResourceType.Type,
				Source: parentResourceType.Source,

				ImageSpecFrom: subPlanID,
				BaseImageType: subBaseImageType,

				VersionedResourceTypes: trimmedResourceTypes,

				Tags: tags,
			})

			imageGetConfig.VersionFrom = &checkPlan.ID
			imageGetPlan = planFactory.NewPlan(imageGetConfig)

			// Build plan for running the check and then the get for the image
			imageFetchPlan = planFactory.NewPlan(atc.OnSuccessPlan{
				Step: checkPlan,
				Next: imageGetPlan,
			})
		} else {
			// version is already provided, only need to do Get step
			imageGetConfig.Version = &resourceTypeVersion
			imageGetPlan = planFactory.NewPlan(imageGetConfig)

			imageFetchPlan = imageGetPlan
		}

		if subBaseImageType == nil {
			// If the parent of this resource type is a custom type,
			// we have nested resource-types and need to run substeps first
			return planFactory.NewPlan(atc.OnSuccessPlan{
				Step: subPlan,
				Next: imageFetchPlan,
			}), &imageGetPlan.ID, nil
		} else {
			// The parent of this resource type is a base type so we don't need to
			// run any substeps first
			return imageFetchPlan, &imageGetPlan.ID, nil
		}
	} else {
		// This resource type is a base type, no need to fetch image
		return atc.Plan{}, nil, &resourceTypeName
	}
}
