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
		Name:              step.Name,
		Privileged:        step.Privileged,
		Config:            step.Config,
		ConfigPath:        step.ConfigPath,
		Vars:              step.Vars,
		Tags:              step.Tags,
		Params:            step.Params,
		InputMapping:      step.InputMapping,
		OutputMapping:     step.OutputMapping,
		ImageArtifactName: step.ImageArtifactName,
		Timeout:           step.Timeout,

		VersionedResourceTypes: visitor.resourceTypes,
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

	var privileged bool
	parentResourceType, found := visitor.resourceTypes.Lookup(resource.Type)
	if found {
		privileged = parentResourceType.Privileged
	}

	plan := visitor.planFactory.NewPlan(atc.GetPlan{
		Name: step.Name,

		Type:       resource.Type,
		Resource:   resourceName,
		Source:     resource.Source,
		Params:     step.Params,
		Version:    &version,
		Tags:       step.Tags,
		Timeout:    step.Timeout,
		Privileged: privileged,

		VersionedResourceTypes: visitor.resourceTypes,
	})

	plan.Get.ImageCheckPlan, plan.Get.ImageGetPlan, plan.Get.BaseType = imageStrategy(plan.ID, resource.Type, visitor.resourceTypes, step.Tags)
	visitor.plan = plan
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

	var privileged bool
	parentResourceType, found := visitor.resourceTypes.Lookup(resource.Type)
	if found {
		privileged = parentResourceType.Privileged
	}

	plan := visitor.planFactory.NewPlan(atc.PutPlan{
		Type:       resource.Type,
		Name:       logicalName,
		Resource:   resourceName,
		Source:     resource.Source,
		Params:     step.Params,
		Tags:       step.Tags,
		Inputs:     step.Inputs,
		Timeout:    step.Timeout,
		Privileged: privileged,

		ExposeBuildCreatedBy:   resource.ExposeBuildCreatedBy,
		VersionedResourceTypes: visitor.resourceTypes,
	})

	plan.Put.ImageCheckPlan, plan.Put.ImageGetPlan, plan.Put.BaseType = imageStrategy(plan.ID, resource.Type, visitor.resourceTypes, step.Tags)

	dependentGetPlan := visitor.planFactory.NewPlan(atc.GetPlan{
		Name:        logicalName,
		Resource:    resourceName,
		Type:        resource.Type,
		Source:      resource.Source,
		VersionFrom: &plan.ID,

		Params:     step.GetParams,
		Tags:       step.Tags,
		Timeout:    step.Timeout,
		Privileged: privileged,

		VersionedResourceTypes: visitor.resourceTypes,
	})
	dependentGetPlan.Get.ImageCheckPlan, dependentGetPlan.Get.ImageGetPlan, dependentGetPlan.Get.BaseType = imageStrategy(dependentGetPlan.ID, resource.Type, visitor.resourceTypes, step.Tags)

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
		vars[i] = atc.AcrossVar(v)
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

func NewCheckPlanner(planFactory atc.PlanFactory) *CheckPlanner {
	return &CheckPlanner{
		planFactory: planFactory,
	}
}

func (c *CheckPlanner) Create(checkable db.Checkable, versionedResourceTypes atc.VersionedResourceTypes, from atc.Version, sourceDefaults atc.Source, interval time.Duration) atc.Plan {
	var privileged bool
	parentResourceType, found := versionedResourceTypes.Lookup(checkable.Type())
	if found {
		privileged = parentResourceType.Privileged
	}

	plan := c.planFactory.NewPlan(atc.CheckPlan{
		Name:    checkable.Name(),
		Type:    checkable.Type(),
		Source:  sourceDefaults.Merge(checkable.Source()),
		Tags:    checkable.Tags(),
		Timeout: checkable.CheckTimeout(),

		FromVersion:            from,
		Interval:               interval.String(),
		VersionedResourceTypes: versionedResourceTypes,
		Resource:               checkable.Name(),
		Privileged:             privileged,
	})

	plan.Check.ImageCheckPlan, plan.Check.ImageGetPlan, plan.Check.BaseType = imageStrategy(plan.ID, checkable.Type(), versionedResourceTypes, checkable.Tags())
	return plan
}

func imageStrategy(planID atc.PlanID, resourceTypeName string, resourceTypes atc.VersionedResourceTypes, stepTags atc.Tags) (*atc.Plan, *atc.Plan, string) {
	// Check if the resource type is a custom resource type
	parentResourceType, found := resourceTypes.Lookup(resourceTypeName)
	if !found {
		// This resource type is a base type, no need to fetch image
		return nil, nil, resourceTypeName
	}

	trimmedResourceTypes := resourceTypes.Without(resourceTypeName)

	image := atc.ImageResource{
		Name:    parentResourceType.Name,
		Type:    parentResourceType.Type,
		Source:  parentResourceType.Source,
		Params:  parentResourceType.Params,
		Version: parentResourceType.Version,
		Tags:    parentResourceType.Tags,
	}
	checkPlan, getPlan, baseType := FetchImagePlan(planID, image, trimmedResourceTypes, stepTags)
	return checkPlan, &getPlan, baseType
}

func FetchImagePlan(planID atc.PlanID, image atc.ImageResource, resourceTypes atc.VersionedResourceTypes, stepTags atc.Tags) (*atc.Plan, atc.Plan, string) {
	// If resource type is a custom type, recurse in order to resolve nested resource types
	getPlanID := planID + "/image-get"
	subCheckPlan, subGetPlan, baseType := imageStrategy(getPlanID, image.Type, resourceTypes, stepTags)

	tags := image.Tags
	if len(image.Tags) == 0 {
		tags = stepTags
	}

	parentResourceType, found := resourceTypes.Lookup(image.Type)
	var privileged bool
	if found {
		privileged = parentResourceType.Privileged
	}

	// Construct get plan for image
	imageGetPlan := atc.Plan{
		ID: getPlanID,
		Get: &atc.GetPlan{
			Name:   image.Name,
			Type:   image.Type,
			Source: image.Source,
			Params: image.Params,

			ImageCheckPlan: subCheckPlan,
			ImageGetPlan:   subGetPlan,
			BaseType:       baseType,

			VersionedResourceTypes: resourceTypes,

			Tags: tags,

			Privileged: privileged,
		},
	}

	resourceTypeVersion := image.Version
	var maybeCheckPlan *atc.Plan
	if resourceTypeVersion == nil {
		checkPlanID := planID + "/image-check"
		subCheckPlan, subGetPlan, baseType = imageStrategy(checkPlanID, image.Type, resourceTypes, stepTags)
		// don't know the version, need to do a Check before the Get
		checkPlan := atc.Plan{
			ID: checkPlanID,
			Check: &atc.CheckPlan{
				Name:   image.Name,
				Type:   image.Type,
				Source: image.Source,

				ImageCheckPlan: subCheckPlan,
				ImageGetPlan:   subGetPlan,
				BaseType:       baseType,

				VersionedResourceTypes: resourceTypes,

				Tags: tags,

				Privileged: privileged,
			},
		}
		maybeCheckPlan = &checkPlan

		imageGetPlan.Get.VersionFrom = &checkPlan.ID
	} else {
		// version is already provided, only need to do Get step
		imageGetPlan.Get.Version = &resourceTypeVersion
	}

	return maybeCheckPlan, imageGetPlan, baseType
}
