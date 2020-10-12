package builder

import (
	"encoding/json"

	"errors"
	"strconv"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec"
)

const supportedSchema = "exec.v2"

//go:generate counterfeiter . StepFactory

type StepFactory interface {
	GetStep(atc.Plan, exec.StepMetadata, db.ContainerMetadata, DelegateFactory) exec.Step
	PutStep(atc.Plan, exec.StepMetadata, db.ContainerMetadata, DelegateFactory) exec.Step
	TaskStep(atc.Plan, exec.StepMetadata, db.ContainerMetadata, DelegateFactory) exec.Step
	CheckStep(atc.Plan, exec.StepMetadata, db.ContainerMetadata, DelegateFactory) exec.Step
	SetPipelineStep(atc.Plan, exec.StepMetadata, DelegateFactory) exec.Step
	LoadVarStep(atc.Plan, exec.StepMetadata, DelegateFactory) exec.Step
	ArtifactInputStep(atc.Plan, db.Build) exec.Step
	ArtifactOutputStep(atc.Plan, db.Build) exec.Step
}

func NewStepBuilder(
	stepFactory StepFactory,
	externalURL string,
	rateLimiter RateLimiter,
) *StepBuilder {
	return &StepBuilder{
		stepFactory: stepFactory,
		externalURL: externalURL,
		rateLimiter: rateLimiter,
	}
}

type StepBuilder struct {
	stepFactory     StepFactory
	delegateFactory DelegateFactory
	externalURL     string
	rateLimiter     RateLimiter
}

func (builder *StepBuilder) BuildStepper(build db.Build) (exec.Stepper, error) {
	if build.Schema() != supportedSchema {
		return nil, errors.New("schema not supported")
	}

	return func(plan atc.Plan) exec.Step {
		return builder.buildStep(build, plan)
	}, nil
}

func (builder *StepBuilder) buildStep(build db.Build, plan atc.Plan) exec.Step {
	if plan.Aggregate != nil {
		return builder.buildAggregateStep(build, plan)
	}

	if plan.InParallel != nil {
		return builder.buildParallelStep(build, plan)
	}

	if plan.Across != nil {
		return builder.buildAcrossStep(build, plan)
	}

	if plan.Do != nil {
		return builder.buildDoStep(build, plan)
	}

	if plan.Timeout != nil {
		return builder.buildTimeoutStep(build, plan)
	}

	if plan.Try != nil {
		return builder.buildTryStep(build, plan)
	}

	if plan.OnAbort != nil {
		return builder.buildOnAbortStep(build, plan)
	}

	if plan.OnError != nil {
		return builder.buildOnErrorStep(build, plan)
	}

	if plan.OnSuccess != nil {
		return builder.buildOnSuccessStep(build, plan)
	}

	if plan.OnFailure != nil {
		return builder.buildOnFailureStep(build, plan)
	}

	if plan.Ensure != nil {
		return builder.buildEnsureStep(build, plan)
	}

	if plan.Task != nil {
		return builder.buildTaskStep(build, plan)
	}

	if plan.SetPipeline != nil {
		return builder.buildSetPipelineStep(build, plan)
	}

	if plan.LoadVar != nil {
		return builder.buildLoadVarStep(build, plan)
	}

	if plan.Check != nil {
		return builder.buildCheckStep(build, plan)
	}

	if plan.Get != nil {
		return builder.buildGetStep(build, plan)
	}

	if plan.Put != nil {
		return builder.buildPutStep(build, plan)
	}

	if plan.Retry != nil {
		return builder.buildRetryStep(build, plan)
	}

	if plan.ArtifactInput != nil {
		return builder.buildArtifactInputStep(build, plan)
	}

	if plan.ArtifactOutput != nil {
		return builder.buildArtifactOutputStep(build, plan)
	}

	return exec.IdentityStep{}
}

func (builder *StepBuilder) buildAggregateStep(build db.Build, plan atc.Plan) exec.Step {

	agg := exec.AggregateStep{}

	for _, innerPlan := range *plan.Aggregate {
		innerPlan.Attempts = plan.Attempts
		step := builder.buildStep(build, innerPlan)
		agg = append(agg, step)
	}

	return agg
}

func (builder *StepBuilder) buildParallelStep(build db.Build, plan atc.Plan) exec.Step {

	var steps []exec.Step

	for _, innerPlan := range plan.InParallel.Steps {
		innerPlan.Attempts = plan.Attempts
		step := builder.buildStep(build, innerPlan)
		steps = append(steps, step)
	}

	return exec.InParallel(steps, plan.InParallel.Limit, plan.InParallel.FailFast)
}

func (builder *StepBuilder) buildAcrossStep(build db.Build, plan atc.Plan) exec.Step {
	stepMetadata := builder.stepMetadata(
		build,
		builder.externalURL,
	)

	steps := make([]exec.ScopedStep, len(plan.Across.Steps))
	for i, s := range plan.Across.Steps {
		steps[i] = exec.ScopedStep{
			Step:   builder.buildStep(build, s.Step),
			Values: s.Values,
		}
	}

	return exec.Across(
		plan.Across.Vars,
		steps,
		plan.Across.FailFast,
		buildDelegateFactory(build, plan, builder.rateLimiter),
		stepMetadata,
	)
}

func (builder *StepBuilder) buildDoStep(build db.Build, plan atc.Plan) exec.Step {
	var step exec.Step = exec.IdentityStep{}

	for i := len(*plan.Do) - 1; i >= 0; i-- {
		innerPlan := (*plan.Do)[i]
		innerPlan.Attempts = plan.Attempts
		previous := builder.buildStep(build, innerPlan)
		step = exec.OnSuccess(previous, step)
	}

	return step
}

func (builder *StepBuilder) buildTimeoutStep(build db.Build, plan atc.Plan) exec.Step {
	innerPlan := plan.Timeout.Step
	innerPlan.Attempts = plan.Attempts
	step := builder.buildStep(build, innerPlan)
	return exec.Timeout(step, plan.Timeout.Duration)
}

func (builder *StepBuilder) buildTryStep(build db.Build, plan atc.Plan) exec.Step {
	innerPlan := plan.Try.Step
	innerPlan.Attempts = plan.Attempts
	step := builder.buildStep(build, innerPlan)
	return exec.Try(step)
}

func (builder *StepBuilder) buildOnAbortStep(build db.Build, plan atc.Plan) exec.Step {
	plan.OnAbort.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.OnAbort.Step)
	plan.OnAbort.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.OnAbort.Next)
	return exec.OnAbort(step, next)
}

func (builder *StepBuilder) buildOnErrorStep(build db.Build, plan atc.Plan) exec.Step {
	plan.OnError.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.OnError.Step)
	plan.OnError.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.OnError.Next)
	return exec.OnError(step, next)
}

func (builder *StepBuilder) buildOnSuccessStep(build db.Build, plan atc.Plan) exec.Step {
	plan.OnSuccess.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.OnSuccess.Step)
	plan.OnSuccess.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.OnSuccess.Next)
	return exec.OnSuccess(step, next)
}

func (builder *StepBuilder) buildOnFailureStep(build db.Build, plan atc.Plan) exec.Step {
	plan.OnFailure.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.OnFailure.Step)
	plan.OnFailure.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.OnFailure.Next)
	return exec.OnFailure(step, next)
}

func (builder *StepBuilder) buildEnsureStep(build db.Build, plan atc.Plan) exec.Step {
	plan.Ensure.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.Ensure.Step)
	plan.Ensure.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.Ensure.Next)
	return exec.Ensure(step, next)
}

func (builder *StepBuilder) buildRetryStep(build db.Build, plan atc.Plan) exec.Step {
	steps := []exec.Step{}

	for index, innerPlan := range *plan.Retry {
		innerPlan.Attempts = append(plan.Attempts, index+1)

		step := builder.buildStep(build, innerPlan)
		steps = append(steps, step)
	}

	return exec.Retry(steps...)
}

func (builder *StepBuilder) buildGetStep(build db.Build, plan atc.Plan) exec.Step {

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
		buildDelegateFactory(build, plan, builder.rateLimiter),
	)
}

func (builder *StepBuilder) buildPutStep(build db.Build, plan atc.Plan) exec.Step {

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
		buildDelegateFactory(build, plan, builder.rateLimiter),
	)
}

func (builder *StepBuilder) buildCheckStep(build db.Build, plan atc.Plan) exec.Step {
	containerMetadata := builder.containerMetadata(
		build,
		db.ContainerTypeCheck,
		plan.Check.Name,
		plan.Attempts,
	)

	stepMetadata := builder.stepMetadata(
		build,
		builder.externalURL,
	)

	return builder.stepFactory.CheckStep(
		plan,
		stepMetadata,
		containerMetadata,
		buildDelegateFactory(build, plan, builder.rateLimiter),
	)
}

func (builder *StepBuilder) buildTaskStep(build db.Build, plan atc.Plan) exec.Step {

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
		buildDelegateFactory(build, plan, builder.rateLimiter),
	)
}

func (builder *StepBuilder) buildSetPipelineStep(build db.Build, plan atc.Plan) exec.Step {

	stepMetadata := builder.stepMetadata(
		build,
		builder.externalURL,
	)

	return builder.stepFactory.SetPipelineStep(
		plan,
		stepMetadata,
		buildDelegateFactory(build, plan, builder.rateLimiter),
	)
}

func (builder *StepBuilder) buildLoadVarStep(build db.Build, plan atc.Plan) exec.Step {

	stepMetadata := builder.stepMetadata(
		build,
		builder.externalURL,
	)

	return builder.stepFactory.LoadVarStep(
		plan,
		stepMetadata,
		buildDelegateFactory(build, plan, builder.rateLimiter),
	)
}

func (builder *StepBuilder) buildArtifactInputStep(build db.Build, plan atc.Plan) exec.Step {
	return builder.stepFactory.ArtifactInputStep(
		plan,
		build,
	)
}

func (builder *StepBuilder) buildArtifactOutputStep(build db.Build, plan atc.Plan) exec.Step {
	return builder.stepFactory.ArtifactOutputStep(
		plan,
		build,
	)
}

func (builder *StepBuilder) containerMetadata(
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

func (builder *StepBuilder) stepMetadata(
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
