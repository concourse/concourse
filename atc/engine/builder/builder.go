package builder

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec"
)

const supportedSchema = "exec.v2"

//go:generate counterfeiter . StepFactory

type StepFactory interface {
	GetStep(atc.Plan, db.Build, exec.StepMetadata, db.ContainerMetadata, exec.GetDelegate) exec.Step
	PutStep(atc.Plan, db.Build, exec.StepMetadata, db.ContainerMetadata, exec.PutDelegate) exec.Step
	TaskStep(atc.Plan, db.Build, db.ContainerMetadata, exec.TaskDelegate) exec.Step
	ArtifactInputStep(atc.Plan, db.Build, exec.BuildStepDelegate) exec.Step
	ArtifactOutputStep(atc.Plan, db.Build, exec.BuildStepDelegate) exec.Step
}

//go:generate counterfeiter . DelegateFactory

type DelegateFactory interface {
	GetDelegate(db.Build, atc.PlanID) exec.GetDelegate
	PutDelegate(db.Build, atc.PlanID) exec.PutDelegate
	TaskDelegate(db.Build, atc.PlanID) exec.TaskDelegate
	BuildStepDelegate(db.Build, atc.PlanID) exec.BuildStepDelegate
}

func NewStepBuilder(
	stepFactory StepFactory,
	delegateFactory DelegateFactory,
	externalURL string,
) *stepBuilder {
	return &stepBuilder{
		stepFactory:     stepFactory,
		delegateFactory: delegateFactory,
		externalURL:     externalURL,
	}
}

type stepBuilder struct {
	stepFactory     StepFactory
	delegateFactory DelegateFactory
	externalURL     string
}

func (builder *stepBuilder) BuildStep(build db.Build) (exec.Step, error) {

	if build == nil {
		return exec.IdentityStep{}, errors.New("Must provide a build")
	}

	if build.Schema() != supportedSchema {
		return exec.IdentityStep{}, errors.New("Schema not supported")
	}

	return builder.buildStep(build, build.PrivatePlan()), nil
}

func (builder *stepBuilder) buildStep(build db.Build, plan atc.Plan) exec.Step {
	if plan.Aggregate != nil {
		return builder.buildAggregateStep(build, plan)
	}

	if plan.InParallel != nil {
		return builder.buildParallelStep(build, plan)
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

func (builder *stepBuilder) buildAggregateStep(build db.Build, plan atc.Plan) exec.Step {

	agg := exec.AggregateStep{}

	for _, innerPlan := range *plan.Aggregate {
		innerPlan.Attempts = plan.Attempts
		step := builder.buildStep(build, innerPlan)
		agg = append(agg, step)
	}

	return agg
}

func (builder *stepBuilder) buildParallelStep(build db.Build, plan atc.Plan) exec.Step {

	var steps []exec.Step

	for _, innerPlan := range plan.InParallel.Steps {
		innerPlan.Attempts = plan.Attempts
		step := builder.buildStep(build, innerPlan)
		steps = append(steps, step)
	}

	return exec.InParallel(steps, plan.InParallel.Limit, plan.InParallel.FailFast)
}

func (builder *stepBuilder) buildDoStep(build db.Build, plan atc.Plan) exec.Step {

	var step exec.Step = exec.IdentityStep{}

	for i := len(*plan.Do) - 1; i >= 0; i-- {
		innerPlan := (*plan.Do)[i]
		innerPlan.Attempts = plan.Attempts
		previous := builder.buildStep(build, innerPlan)
		step = exec.OnSuccess(previous, step)
	}

	return step
}

func (builder *stepBuilder) buildTimeoutStep(build db.Build, plan atc.Plan) exec.Step {
	innerPlan := plan.Timeout.Step
	innerPlan.Attempts = plan.Attempts
	step := builder.buildStep(build, innerPlan)
	return exec.Timeout(step, plan.Timeout.Duration)
}

func (builder *stepBuilder) buildTryStep(build db.Build, plan atc.Plan) exec.Step {
	innerPlan := plan.Try.Step
	innerPlan.Attempts = plan.Attempts
	step := builder.buildStep(build, innerPlan)
	return exec.Try(step)
}

func (builder *stepBuilder) buildOnAbortStep(build db.Build, plan atc.Plan) exec.Step {
	plan.OnAbort.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.OnAbort.Step)
	plan.OnAbort.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.OnAbort.Next)
	return exec.OnAbort(step, next)
}

func (builder *stepBuilder) buildOnErrorStep(build db.Build, plan atc.Plan) exec.Step {
	plan.OnError.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.OnError.Step)
	plan.OnError.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.OnError.Next)
	return exec.OnError(step, next)
}

func (builder *stepBuilder) buildOnSuccessStep(build db.Build, plan atc.Plan) exec.Step {
	plan.OnSuccess.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.OnSuccess.Step)
	plan.OnSuccess.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.OnSuccess.Next)
	return exec.OnSuccess(step, next)
}

func (builder *stepBuilder) buildOnFailureStep(build db.Build, plan atc.Plan) exec.Step {
	plan.OnFailure.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.OnFailure.Step)
	plan.OnFailure.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.OnFailure.Next)
	return exec.OnFailure(step, next)
}

func (builder *stepBuilder) buildEnsureStep(build db.Build, plan atc.Plan) exec.Step {
	plan.Ensure.Step.Attempts = plan.Attempts
	step := builder.buildStep(build, plan.Ensure.Step)
	plan.Ensure.Next.Attempts = plan.Attempts
	next := builder.buildStep(build, plan.Ensure.Next)
	return exec.Ensure(step, next)
}

func (builder *stepBuilder) buildRetryStep(build db.Build, plan atc.Plan) exec.Step {
	steps := []exec.Step{}

	for index, innerPlan := range *plan.Retry {
		innerPlan.Attempts = append(plan.Attempts, index+1)

		step := builder.buildStep(build, innerPlan)
		steps = append(steps, step)
	}

	return exec.Retry(steps...)
}

func (builder *stepBuilder) buildGetStep(build db.Build, plan atc.Plan) exec.Step {

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
		build,
		stepMetadata,
		containerMetadata,
		builder.delegateFactory.GetDelegate(build, plan.ID),
	)
}

func (builder *stepBuilder) buildPutStep(build db.Build, plan atc.Plan) exec.Step {

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
		build,
		stepMetadata,
		containerMetadata,
		builder.delegateFactory.PutDelegate(build, plan.ID),
	)
}

func (builder *stepBuilder) buildTaskStep(build db.Build, plan atc.Plan) exec.Step {

	containerMetadata := builder.containerMetadata(
		build,
		db.ContainerTypeTask,
		plan.Task.Name,
		plan.Attempts,
	)

	return builder.stepFactory.TaskStep(
		plan,
		build,
		containerMetadata,
		builder.delegateFactory.TaskDelegate(build, plan.ID),
	)
}

func (builder *stepBuilder) buildArtifactInputStep(build db.Build, plan atc.Plan) exec.Step {

	return builder.stepFactory.ArtifactInputStep(
		plan,
		build,
		builder.delegateFactory.BuildStepDelegate(build, plan.ID),
	)
}

func (builder *stepBuilder) buildArtifactOutputStep(build db.Build, plan atc.Plan) exec.Step {

	return builder.stepFactory.ArtifactOutputStep(
		plan,
		build,
		builder.delegateFactory.BuildStepDelegate(build, plan.ID),
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

	return db.ContainerMetadata{
		Type: containerType,

		PipelineID: build.PipelineID(),
		JobID:      build.JobID(),
		BuildID:    build.ID(),

		PipelineName: build.PipelineName(),
		JobName:      build.JobName(),
		BuildName:    build.Name(),

		StepName: stepName,
		Attempt:  strings.Join(attemptStrs, "."),
	}
}

func (builder *stepBuilder) stepMetadata(
	build db.Build,
	externalURL string,
) StepMetadata {
	return StepMetadata{
		BuildID:      build.ID(),
		BuildName:    build.Name(),
		JobName:      build.JobName(),
		PipelineName: build.PipelineName(),
		TeamName:     build.TeamName(),
		ExternalURL:  externalURL,
	}
}

type StepMetadata struct {
	BuildID int

	PipelineName string
	JobName      string
	BuildName    string
	ExternalURL  string
	TeamName     string
}

func (metadata StepMetadata) Env() []string {
	env := []string{fmt.Sprintf("BUILD_ID=%d", metadata.BuildID)}

	if metadata.PipelineName != "" {
		env = append(env, "BUILD_PIPELINE_NAME="+metadata.PipelineName)
	}

	if metadata.JobName != "" {
		env = append(env, "BUILD_JOB_NAME="+metadata.JobName)
	}

	if metadata.BuildName != "" {
		env = append(env, "BUILD_NAME="+metadata.BuildName)
	}

	if metadata.ExternalURL != "" {
		env = append(env, "ATC_EXTERNAL_URL="+metadata.ExternalURL)
	}

	if metadata.TeamName != "" {
		env = append(env, "BUILD_TEAM_NAME="+metadata.TeamName)
	}

	return env
}
