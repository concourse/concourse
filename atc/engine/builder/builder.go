package builder

import (
	"code.cloudfoundry.org/lager"
	"encoding/json"

	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/vars"
)

const supportedSchema = "exec.v2"

//go:generate counterfeiter . StepFactory

type StepFactory interface {
	GetStep(atc.Plan, exec.StepMetadata, db.ContainerMetadata, exec.GetDelegate) exec.Step
	PutStep(atc.Plan, exec.StepMetadata, db.ContainerMetadata, exec.PutDelegate) exec.Step
	TaskStep(atc.Plan, exec.StepMetadata, db.ContainerMetadata, exec.TaskDelegate) exec.Step
	CheckStep(atc.Plan, exec.StepMetadata, db.ContainerMetadata, exec.CheckDelegate) exec.Step
	SetPipelineStep(atc.Plan, exec.StepMetadata, exec.SetPipelineStepDelegate) exec.Step
	LoadVarStep(atc.Plan, exec.StepMetadata, exec.BuildStepDelegate) exec.Step
	ArtifactInputStep(atc.Plan, db.Build, exec.BuildStepDelegate) exec.Step
	ArtifactOutputStep(atc.Plan, db.Build, exec.BuildStepDelegate) exec.Step
}

//go:generate counterfeiter . DelegateFactory

type DelegateFactory interface {
	GetDelegate(db.Build, atc.PlanID, *vars.BuildVariables) exec.GetDelegate
	PutDelegate(db.Build, atc.PlanID, *vars.BuildVariables) exec.PutDelegate
	TaskDelegate(db.Build, atc.PlanID, *vars.BuildVariables) exec.TaskDelegate
	CheckDelegate(db.Check, atc.PlanID, *vars.BuildVariables) exec.CheckDelegate
	BuildStepDelegate(db.Build, atc.PlanID, *vars.BuildVariables) exec.BuildStepDelegate
	SetPipelineStepDelegate(db.Build, atc.PlanID, *vars.BuildVariables) exec.SetPipelineStepDelegate
}

func NewStepBuilder(
	stepFactory StepFactory,
	delegateFactory DelegateFactory,
	externalURL string,
	secrets creds.Secrets,
	varSourcePool creds.VarSourcePool,
) *stepBuilder {
	return &stepBuilder{
		stepFactory:     stepFactory,
		delegateFactory: delegateFactory,
		externalURL:     externalURL,
		globalSecrets:   secrets,
		varSourcePool:   varSourcePool,
	}
}

type stepBuilder struct {
	stepFactory     StepFactory
	delegateFactory DelegateFactory
	externalURL     string
	globalSecrets   creds.Secrets
	varSourcePool   creds.VarSourcePool
}

func (builder *stepBuilder) BuildStep(logger lager.Logger, build db.Build) (exec.Step, error) {
	if build == nil {
		return exec.IdentityStep{}, errors.New("must provide a build")
	}

	if build.Schema() != supportedSchema {
		return exec.IdentityStep{}, errors.New("schema not supported")
	}

	var buildVars *vars.BuildVariables

	// "fly execute" generated build will have no pipeline.
	if build.PipelineID() == 0 {
		globalVars := creds.NewVariables(builder.globalSecrets, build.TeamName(), build.PipelineName(), false)
		buildVars = vars.NewBuildVariables(globalVars, atc.EnableRedactSecrets)
	} else {
		pipeline, found, err := build.Pipeline()
		if err != nil {
			return exec.IdentityStep{}, errors.New(fmt.Sprintf("failed to find pipeline: %s", err.Error()))
		}
		if !found {
			return exec.IdentityStep{}, errors.New("pipeline not found")
		}

		varss, err := pipeline.Variables(logger, builder.globalSecrets, builder.varSourcePool)
		if err != nil {
			return exec.IdentityStep{}, err
		}
		buildVars = vars.NewBuildVariables(varss, atc.EnableRedactSecrets)
	}

	return builder.buildStep(build, build.PrivatePlan(), buildVars), nil
}

func (builder *stepBuilder) BuildStepErrored(logger lager.Logger, build db.Build, err error) {
	builder.delegateFactory.BuildStepDelegate(build, build.PrivatePlan().ID, nil).Errored(logger, err.Error())
}

func (builder *stepBuilder) CheckStep(logger lager.Logger, check db.Check) (exec.Step, error) {

	if check == nil {
		return exec.IdentityStep{}, errors.New("must provide a check")
	}

	if check.Schema() != supportedSchema {
		return exec.IdentityStep{}, errors.New("schema not supported")
	}

	pipeline, found, err := check.Pipeline()
	if err != nil {
		return exec.IdentityStep{}, errors.New(fmt.Sprintf("failed to find pipeline: %s", err.Error()))
	}
	if !found {
		return exec.IdentityStep{}, errors.New("pipeline not found")
	}

	varss, err := pipeline.Variables(logger, builder.globalSecrets, builder.varSourcePool)
	if err != nil {
		return exec.IdentityStep{}, fmt.Errorf("failed to create pipeline variables: %s", err.Error())
	}
	buildVars := vars.NewBuildVariables(varss, atc.EnableRedactSecrets)
	return builder.buildCheckStep(check, check.Plan(), buildVars), nil
}

func (builder *stepBuilder) buildStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {
	if plan.Aggregate != nil {
		return builder.buildAggregateStep(build, plan, buildVars)
	}

	if plan.InParallel != nil {
		return builder.buildParallelStep(build, plan, buildVars)
	}

	if plan.Across != nil {
		return builder.buildAcrossStep(build, plan, buildVars)
	}

	if plan.Do != nil {
		return builder.buildDoStep(build, plan, buildVars)
	}

	if plan.Timeout != nil {
		return builder.buildTimeoutStep(build, plan, buildVars)
	}

	if plan.Try != nil {
		return builder.buildTryStep(build, plan, buildVars)
	}

	if plan.OnAbort != nil {
		return builder.buildOnAbortStep(build, plan, buildVars)
	}

	if plan.OnError != nil {
		return builder.buildOnErrorStep(build, plan, buildVars)
	}

	if plan.OnSuccess != nil {
		return builder.buildOnSuccessStep(build, plan, buildVars)
	}

	if plan.OnFailure != nil {
		return builder.buildOnFailureStep(build, plan, buildVars)
	}

	if plan.Ensure != nil {
		return builder.buildEnsureStep(build, plan, buildVars)
	}

	if plan.Task != nil {
		return builder.buildTaskStep(build, plan, buildVars)
	}

	if plan.SetPipeline != nil {
		return builder.buildSetPipelineStep(build, plan, buildVars)
	}

	if plan.LoadVar != nil {
		return builder.buildLoadVarStep(build, plan, buildVars)
	}

	if plan.Get != nil {
		return builder.buildGetStep(build, plan, buildVars)
	}

	if plan.Put != nil {
		return builder.buildPutStep(build, plan, buildVars)
	}

	if plan.Retry != nil {
		return builder.buildRetryStep(build, plan, buildVars)
	}

	if plan.ArtifactInput != nil {
		return builder.buildArtifactInputStep(build, plan, buildVars)
	}

	if plan.ArtifactOutput != nil {
		return builder.buildArtifactOutputStep(build, plan, buildVars)
	}

	return exec.IdentityStep{}
}

func (builder *stepBuilder) buildAggregateStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {

	agg := exec.AggregateStep{}

	for _, innerPlan := range *plan.Aggregate {
		innerPlan.Attempts = plan.Attempts
		step := builder.buildStep(build, innerPlan, buildVars)
		agg = append(agg, step)
	}

	return agg
}

func (builder *stepBuilder) buildParallelStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {

	var steps []exec.Step

	for _, innerPlan := range plan.InParallel.Steps {
		innerPlan.Attempts = plan.Attempts
		step := builder.buildStep(build, innerPlan, buildVars)
		steps = append(steps, step)
	}

	return exec.InParallel(steps, plan.InParallel.Limit, plan.InParallel.FailFast)
}

func (builder *stepBuilder) buildAcrossStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {
	step := builder.buildAcrossInParallelStep(build, 0, *plan.Across, buildVars)

	stepMetadata := builder.stepMetadata(
		build,
		builder.externalURL,
	)

	varNames := make([]string, len(plan.Across.Vars))
	for i, v := range plan.Across.Vars {
		varNames[i] = v.Var
	}

	return exec.Across(
		step,
		varNames,
		builder.delegateFactory.BuildStepDelegate(build, plan.ID, buildVars),
		stepMetadata,
	)
}

func (builder *stepBuilder) buildAcrossInParallelStep(build db.Build, varIndex int, plan atc.AcrossPlan, buildVars *vars.BuildVariables) exec.InParallelStep {
	if varIndex == len(plan.Vars)-1 {
		var steps []exec.Step
		for _, step := range plan.Steps {
			scopedBuildVars := buildVars.NewLocalScope()
			for i, v := range plan.Vars {
				// Don't redact because the `list` operation of a var_source should return identifiers
				// which should be publicly accessible. For static across steps, the static list is
				// embedded directly in the pipeline
				scopedBuildVars.AddLocalVar(v.Var, step.Values[i], false)
			}
			steps = append(steps, builder.buildStep(build, step.Step, scopedBuildVars))
		}
		return exec.InParallel(steps, plan.Vars[varIndex].MaxInFlight, plan.FailFast)
	}
	stepsPerValue := 1
	for _, v := range plan.Vars[varIndex+1:] {
		stepsPerValue *= len(v.Values)
	}
	numValues := len(plan.Vars[varIndex].Values)
	substeps := make([]exec.Step, numValues)
	for i := range substeps {
		startIndex := i * stepsPerValue
		endIndex := (i + 1) * stepsPerValue
		planCopy := plan
		planCopy.Steps = plan.Steps[startIndex:endIndex]
		substeps[i] = builder.buildAcrossInParallelStep(build, varIndex+1, planCopy, buildVars)
	}
	return exec.InParallel(substeps, plan.Vars[varIndex].MaxInFlight, plan.FailFast)
}

func (builder *stepBuilder) buildDoStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {

	var step exec.Step = exec.IdentityStep{}

	for i := len(*plan.Do) - 1; i >= 0; i-- {
		innerPlan := (*plan.Do)[i]
		innerPlan.Attempts = plan.Attempts
		previous := builder.buildStep(build, innerPlan, buildVars)
		step = exec.OnSuccess(previous, step)
	}

	return step
}

func (builder *stepBuilder) buildTimeoutStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {
	innerPlan := plan.Timeout.Step
	innerPlan.Attempts = plan.Attempts
	step := builder.buildStep(build, innerPlan, buildVars)
	return exec.Timeout(step, plan.Timeout.Duration)
}

func (builder *stepBuilder) buildTryStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {
	innerPlan := plan.Try.Step
	innerPlan.Attempts = plan.Attempts
	step := builder.buildStep(build, innerPlan, buildVars)
	return exec.Try(step)
}

func (builder *stepBuilder) buildOnAbortStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {
	plan.OnAbort.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.OnAbort.Step, buildVars)
	plan.OnAbort.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.OnAbort.Next, buildVars)
	return exec.OnAbort(step, next)
}

func (builder *stepBuilder) buildOnErrorStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {
	plan.OnError.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.OnError.Step, buildVars)
	plan.OnError.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.OnError.Next, buildVars)
	return exec.OnError(step, next)
}

func (builder *stepBuilder) buildOnSuccessStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {
	plan.OnSuccess.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.OnSuccess.Step, buildVars)
	plan.OnSuccess.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.OnSuccess.Next, buildVars)
	return exec.OnSuccess(step, next)
}

func (builder *stepBuilder) buildOnFailureStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {
	plan.OnFailure.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.OnFailure.Step, buildVars)
	plan.OnFailure.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.OnFailure.Next, buildVars)
	return exec.OnFailure(step, next)
}

func (builder *stepBuilder) buildEnsureStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {
	plan.Ensure.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.Ensure.Step, buildVars)
	plan.Ensure.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.Ensure.Next, buildVars)
	return exec.Ensure(step, next)
}

func (builder *stepBuilder) buildRetryStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {
	steps := []exec.Step{}

	for index, innerPlan := range *plan.Retry {
		innerPlan.Attempts = append(plan.Attempts, index+1)

		step := builder.buildStep(build, innerPlan, buildVars)
		steps = append(steps, step)
	}

	return exec.Retry(steps...)
}

func (builder *stepBuilder) buildGetStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {

	containerMetadata := builder.containerMetadata(
		build,
		db.ContainerTypeGet,
		plan.Get.Name,
		plan.Attempts,
	)

	stepMetadata := builder.stepMetadata(
		build,
		builder.externalURL,
	)

	return builder.stepFactory.GetStep(
		plan,
		stepMetadata,
		containerMetadata,
		builder.delegateFactory.GetDelegate(build, plan.ID, buildVars),
	)
}

func (builder *stepBuilder) buildPutStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {

	containerMetadata := builder.containerMetadata(
		build,
		db.ContainerTypePut,
		plan.Put.Name,
		plan.Attempts,
	)

	stepMetadata := builder.stepMetadata(
		build,
		builder.externalURL,
	)

	return builder.stepFactory.PutStep(
		plan,
		stepMetadata,
		containerMetadata,
		builder.delegateFactory.PutDelegate(build, plan.ID, buildVars),
	)
}

func (builder *stepBuilder) buildCheckStep(check db.Check, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {

	containerMetadata := db.ContainerMetadata{
		Type: db.ContainerTypeCheck,
	}

	stepMetadata := exec.StepMetadata{
		TeamID:                check.TeamID(),
		TeamName:              check.TeamName(),
		PipelineID:            check.PipelineID(),
		PipelineName:          check.PipelineName(),
		PipelineInstanceVars:  check.PipelineInstanceVars(),
		ResourceConfigScopeID: check.ResourceConfigScopeID(),
		ResourceConfigID:      check.ResourceConfigID(),
		BaseResourceTypeID:    check.BaseResourceTypeID(),
		ExternalURL:           builder.externalURL,
	}

	return builder.stepFactory.CheckStep(
		plan,
		stepMetadata,
		containerMetadata,
		builder.delegateFactory.CheckDelegate(check, plan.ID, buildVars),
	)
}

func (builder *stepBuilder) buildTaskStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {

	containerMetadata := builder.containerMetadata(
		build,
		db.ContainerTypeTask,
		plan.Task.Name,
		plan.Attempts,
	)

	stepMetadata := builder.stepMetadata(
		build,
		builder.externalURL,
	)

	return builder.stepFactory.TaskStep(
		plan,
		stepMetadata,
		containerMetadata,
		builder.delegateFactory.TaskDelegate(build, plan.ID, buildVars),
	)
}

func (builder *stepBuilder) buildSetPipelineStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {

	stepMetadata := builder.stepMetadata(
		build,
		builder.externalURL,
	)

	return builder.stepFactory.SetPipelineStep(
		plan,
		stepMetadata,
		builder.delegateFactory.SetPipelineStepDelegate(build, plan.ID, buildVars),
	)
}

func (builder *stepBuilder) buildLoadVarStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {

	stepMetadata := builder.stepMetadata(
		build,
		builder.externalURL,
	)

	return builder.stepFactory.LoadVarStep(
		plan,
		stepMetadata,
		builder.delegateFactory.BuildStepDelegate(build, plan.ID, buildVars),
	)
}

func (builder *stepBuilder) buildArtifactInputStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {

	return builder.stepFactory.ArtifactInputStep(
		plan,
		build,
		builder.delegateFactory.BuildStepDelegate(build, plan.ID, buildVars),
	)
}

func (builder *stepBuilder) buildArtifactOutputStep(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables) exec.Step {

	return builder.stepFactory.ArtifactOutputStep(
		plan,
		build,
		builder.delegateFactory.BuildStepDelegate(build, plan.ID, buildVars),
	)
}

func (builder *stepBuilder) containerMetadata(
	build db.Build,
	containerType db.ContainerType,
	stepName string,
	attempts []int,
) db.ContainerMetadata {
	attemptStrs := []string{}
	for _, a := range attempts {
		attemptStrs = append(attemptStrs, strconv.Itoa(a))
	}

	var pipelineInstanceVars string
	if build.PipelineInstanceVars() != nil {
		instanceVars, _ := json.Marshal(build.PipelineInstanceVars())
		pipelineInstanceVars = string(instanceVars)
	}

	return db.ContainerMetadata{
		Type: containerType,

		PipelineID: build.PipelineID(),
		JobID:      build.JobID(),
		BuildID:    build.ID(),

		PipelineName:         build.PipelineName(),
		PipelineInstanceVars: pipelineInstanceVars,
		JobName:              build.JobName(),
		BuildName:            build.Name(),

		StepName: stepName,
		Attempt:  strings.Join(attemptStrs, "."),
	}
}

func (builder *stepBuilder) stepMetadata(
	build db.Build,
	externalURL string,
) exec.StepMetadata {
	return exec.StepMetadata{
		BuildID:              build.ID(),
		BuildName:            build.Name(),
		TeamID:               build.TeamID(),
		TeamName:             build.TeamName(),
		JobID:                build.JobID(),
		JobName:              build.JobName(),
		PipelineID:           build.PipelineID(),
		PipelineName:         build.PipelineName(),
		PipelineInstanceVars: build.PipelineInstanceVars(),
		ExternalURL:          externalURL,
	}
}
