package builds

import (
	"fmt"

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
	resourceTypes atc.ResourceTypes,
	varSourceConfigs atc.VarSourceConfigs,
	inputs []db.BuildInput,
) (atc.Plan, error) {
	visitor := &planVisitor{
		planFactory: planner.planFactory,

		resources:        resources,
		resourceTypes:    resourceTypes,
		varSourceConfigs: varSourceConfigs,
		inputs:           inputs,
	}

	err := planConfig.Visit(visitor)
	if err != nil {
		return atc.Plan{}, err
	}

	return visitor.plan, nil
}

type planVisitor struct {
	planFactory atc.PlanFactory

	resources        db.SchedulerResources
	resourceTypes    atc.ResourceTypes
	varSourceConfigs atc.VarSourceConfigs
	inputs           []db.BuildInput

	plan atc.Plan
}

func (visitor *planVisitor) VisitTask(step *atc.TaskStep) error {
	visitor.plan = visitor.planFactory.NewPlan(atc.TaskPlan{
		Name:              step.Name,
		Privileged:        step.Privileged,
		Config:            step.Config,
		Limits:            step.Limits,
		ConfigPath:        step.ConfigPath,
		Vars:              step.Vars,
		Tags:              step.Tags,
		Params:            step.Params,
		InputMapping:      step.InputMapping,
		OutputMapping:     step.OutputMapping,
		ImageArtifactName: step.ImageArtifactName,
		Timeout:           step.Timeout,

		ResourceTypes:    visitor.resourceTypes,
		VarSourceConfigs: visitor.varSourceConfigs,
	})

	return nil
}

func (visitor *planVisitor) VisitGetVar(step *atc.GetVarStep) error {
	varSourceConfig, found := visitor.varSourceConfigs.Lookup(step.Source)
	if !found {
		return atc.UnknownVarSourceError{VarSource: step.Source}
	}

	plan := visitor.planFactory.NewPlan(atc.GetVarPlan{
		Name:   step.Source,
		Path:   step.Name,
		Type:   varSourceConfig.Type,
		Source: varSourceConfig.Config,
	})
	var err error
	plan.GetVar.VarPlans, err = visitor.varSourceConfigs.Without(step.Source).GetVarPlans(plan.ID+"/source", varSourceConfig.Config)
	if err != nil {
		return err
	}

	visitor.plan = plan
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

	plan := visitor.planFactory.NewPlan(atc.GetPlan{
		Name: step.Name,

		Type:     resource.Type,
		Resource: resourceName,
		Source:   resource.Source,
		Params:   step.Params,
		Version:  &version,
		Tags:     step.Tags,
		Timeout:  step.Timeout,
	})

	var err error
	plan.Get.TypeImage, err = visitor.resourceTypes.ImageForType(plan.ID, resource.Type, visitor.varSourceConfigs, step.Tags, false)
	if err != nil {
		return fmt.Errorf("get image planner: %w", err)
	}

	sourceVarPlans, err := visitor.varSourceConfigs.GetVarPlans(plan.ID+"/source", resource.Source)
	if err != nil {
		return fmt.Errorf("get source var plan: %w", err)
	}

	paramsVarPlans, err := visitor.varSourceConfigs.GetVarPlans(plan.ID+"/params", step.Params)
	if err != nil {
		return fmt.Errorf("get params var plan: %w", err)
	}

	plan.Get.VarPlans = append(sourceVarPlans, paramsVarPlans...)

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

	plan := visitor.planFactory.NewPlan(atc.PutPlan{
		Type:     resource.Type,
		Name:     logicalName,
		Resource: resourceName,
		Source:   resource.Source,
		Params:   step.Params,
		Tags:     step.Tags,
		Inputs:   step.Inputs,
		Timeout:  step.Timeout,

		ExposeBuildCreatedBy: resource.ExposeBuildCreatedBy,
	})

	var err error
	plan.Put.TypeImage, err = visitor.resourceTypes.ImageForType(plan.ID, resource.Type, visitor.varSourceConfigs, step.Tags, false)
	if err != nil {
		return fmt.Errorf("put image planner: %w", err)
	}

	putSourceVarPlans, err := visitor.varSourceConfigs.GetVarPlans(plan.ID+"/source", resource.Source)
	if err != nil {
		return fmt.Errorf("put source var plan: %w", err)
	}

	putParamsVarPlans, err := visitor.varSourceConfigs.GetVarPlans(plan.ID+"/params", step.Params)
	if err != nil {
		return fmt.Errorf("put params var plan: %w", err)
	}

	plan.Put.VarPlans = append(putSourceVarPlans, putParamsVarPlans...)

	dependentGetPlan := visitor.planFactory.NewPlan(atc.GetPlan{
		Name:        logicalName,
		Resource:    resourceName,
		Type:        resource.Type,
		Source:      resource.Source,
		VersionFrom: &plan.ID,

		Params:  step.GetParams,
		Tags:    step.Tags,
		Timeout: step.Timeout,
	})

	dependentGetPlan.Get.TypeImage, err = visitor.resourceTypes.ImageForType(dependentGetPlan.ID, resource.Type, visitor.varSourceConfigs, step.Tags, false)
	if err != nil {
		return fmt.Errorf("dependent get image planner: %w", err)
	}

	dependentGetVarSourcePlan, err := visitor.varSourceConfigs.GetVarPlans(dependentGetPlan.ID+"/source", resource.Source)
	if err != nil {
		return fmt.Errorf("dependent get source var plan: %w", err)
	}

	dependentGetVarParamsPlan, err := visitor.varSourceConfigs.GetVarPlans(dependentGetPlan.ID+"/params", step.GetParams)
	if err != nil {
		return fmt.Errorf("dependent get params var plan: %w", err)
	}
	dependentGetPlan.Get.VarPlans = append(dependentGetVarSourcePlan, dependentGetVarParamsPlan...)

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

func FetchImagePlan(planID atc.PlanID, image atc.ImageResource, resourceTypes atc.ResourceTypes, varSourceConfigs atc.VarSourceConfigs, stepTags atc.Tags) (atc.Plan, *atc.Plan, error) {
	// If resource type is a custom type, recurse in order to resolve nested resource types
	getPlanID := planID + "/image-get"

	tags := image.Tags
	if len(image.Tags) == 0 {
		tags = stepTags
	}

	getSourceVarPlans, err := varSourceConfigs.GetVarPlans(getPlanID+"/source", image.Source)
	if err != nil {
		return atc.Plan{}, nil, fmt.Errorf("get source var plan: %w", err)
	}

	getParamsVarPlans, err := varSourceConfigs.GetVarPlans(getPlanID+"/params", image.Params)
	if err != nil {
		return atc.Plan{}, nil, fmt.Errorf("get params var plan: %w", err)
	}

	getTypeImage, err := resourceTypes.ImageForType(getPlanID, image.Type, varSourceConfigs, tags, false)
	if err != nil {
		return atc.Plan{}, nil, fmt.Errorf("get image planner: %w", err)
	}

	// Construct get plan for image
	imageGetPlan := atc.Plan{
		ID: getPlanID,
		Get: &atc.GetPlan{
			Name:   image.Name,
			Type:   image.Type,
			Source: image.Source,
			Params: image.Params,

			TypeImage: getTypeImage,

			VarPlans: append(getSourceVarPlans, getParamsVarPlans...),

			Tags: tags,
		},
	}

	var maybeCheckPlan *atc.Plan
	if image.Version == nil {
		checkPlanID := planID + "/image-check"

		checkSourceVarPlans, err := varSourceConfigs.GetVarPlans(checkPlanID+"/source", image.Source)
		if err != nil {
			return atc.Plan{}, nil, fmt.Errorf("check source var plan: %w", err)
		}

		checkTypeImage, err := resourceTypes.ImageForType(checkPlanID, image.Type, varSourceConfigs, tags, false)
		if err != nil {
			return atc.Plan{}, nil, fmt.Errorf("check image planner: %w", err)
		}

		// don't know the version, need to do a Check before the Get
		checkPlan := atc.Plan{
			ID: checkPlanID,
			Check: &atc.CheckPlan{
				Name:   image.Name,
				Type:   image.Type,
				Source: image.Source,

				TypeImage: checkTypeImage,

				VarPlans: checkSourceVarPlans,

				Tags: tags,
			},
		}
		maybeCheckPlan = &checkPlan

		imageGetPlan.Get.VersionFrom = &checkPlan.ID
	} else {
		// version is already provided, only need to do Get step
		imageGetPlan.Get.Version = &image.Version
	}

	return imageGetPlan, maybeCheckPlan, nil
}
