package engine

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/policy"
)

const supportedSchema = "exec.v2"

//counterfeiter:generate . CoreStepFactory
type CoreStepFactory interface {
	GetStep(atc.Plan, exec.StepMetadata, db.ContainerMetadata, DelegateFactory) exec.Step
	PutStep(atc.Plan, exec.StepMetadata, db.ContainerMetadata, DelegateFactory) exec.Step
	TaskStep(atc.Plan, exec.StepMetadata, db.ContainerMetadata, DelegateFactory) exec.Step
	RunStep(atc.Plan, exec.StepMetadata, db.ContainerMetadata, DelegateFactory) exec.Step
	CheckStep(atc.Plan, exec.StepMetadata, db.ContainerMetadata, DelegateFactory) exec.Step
	SetPipelineStep(atc.Plan, exec.StepMetadata, DelegateFactory) exec.Step
	LoadVarStep(atc.Plan, exec.StepMetadata, DelegateFactory) exec.Step
	ArtifactInputStep(atc.Plan, db.Build) exec.Step
	ArtifactOutputStep(atc.Plan, db.Build) exec.Step
}

//counterfeiter:generate . StepperFactory
type StepperFactory interface {
	StepperForBuild(db.Build) (exec.Stepper, error)
}

func NewStepperFactory(
	coreFactory CoreStepFactory,
	externalURL string,
	rateLimiter RateLimiter,
	policyChecker policy.Checker,
	dbWorkerFactory db.WorkerFactory,
	lockFactory lock.LockFactory,
) StepperFactory {
	return &stepperFactory{
		coreFactory:     coreFactory,
		externalURL:     externalURL,
		rateLimiter:     rateLimiter,
		policyChecker:   policyChecker,
		dbWorkerFactory: dbWorkerFactory,
		lockFactory:     lockFactory,
	}
}

type stepperFactory struct {
	coreFactory     CoreStepFactory
	externalURL     string
	rateLimiter     RateLimiter
	policyChecker   policy.Checker
	dbWorkerFactory db.WorkerFactory
	lockFactory     lock.LockFactory
}

type planBuilder struct {
	factory     *stepperFactory
	build       db.Build
	buildStatus string
}

func (pb *planBuilder) withBuildStatus(state string) *planBuilder {
	return &planBuilder{factory: pb.factory, build: pb.build, buildStatus: state}
}

func (factory *stepperFactory) StepperForBuild(build db.Build) (exec.Stepper, error) {
	if build.Schema() != supportedSchema {
		return nil, errors.New("schema not supported")
	}

	pb := &planBuilder{factory: factory, build: build}
	return func(plan atc.Plan) exec.Step {
		return pb.buildStep(plan)
	}, nil
}

func (pb *planBuilder) buildDelegateFactory(plan atc.Plan) DelegateFactory {
	return DelegateFactory{
		build:           pb.build,
		plan:            plan,
		rateLimiter:     pb.factory.rateLimiter,
		policyChecker:   pb.factory.policyChecker,
		dbWorkerFactory: pb.factory.dbWorkerFactory,
		lockFactory:     pb.factory.lockFactory,
	}
}

func (pb *planBuilder) buildStep(plan atc.Plan) exec.Step {
	if plan.InParallel != nil {
		return pb.buildParallelStep(plan)
	}

	if plan.Across != nil {
		return pb.buildAcrossStep(plan)
	}

	if plan.Do != nil {
		return pb.buildDoStep(plan)
	}

	if plan.Timeout != nil {
		return pb.buildTimeoutStep(plan)
	}

	if plan.Try != nil {
		return pb.buildTryStep(plan)
	}

	if plan.OnAbort != nil {
		return pb.buildOnAbortStep(plan)
	}

	if plan.OnError != nil {
		return pb.buildOnErrorStep(plan)
	}

	if plan.OnSuccess != nil {
		return pb.buildOnSuccessStep(plan)
	}

	if plan.OnFailure != nil {
		return pb.buildOnFailureStep(plan)
	}

	if plan.Ensure != nil {
		return pb.buildEnsureStep(plan)
	}

	if plan.Run != nil {
		return pb.buildRunStep(plan)
	}

	if plan.Task != nil {
		return pb.buildTaskStep(plan)
	}

	if plan.SetPipeline != nil {
		return pb.buildSetPipelineStep(plan)
	}

	if plan.LoadVar != nil {
		return pb.buildLoadVarStep(plan)
	}

	if plan.Check != nil {
		return pb.buildCheckStep(plan)
	}

	if plan.Get != nil {
		return pb.buildGetStep(plan)
	}

	if plan.Put != nil {
		return pb.buildPutStep(plan)
	}

	if plan.Retry != nil {
		return pb.buildRetryStep(plan)
	}

	if plan.ArtifactInput != nil {
		return pb.buildArtifactInputStep(plan)
	}

	if plan.ArtifactOutput != nil {
		return pb.buildArtifactOutputStep(plan)
	}

	return exec.IdentityStep{}
}

func (pb *planBuilder) buildParallelStep(plan atc.Plan) exec.Step {

	var steps []exec.Step

	for _, innerPlan := range plan.InParallel.Steps {
		innerPlan.Attempts = plan.Attempts
		step := pb.buildStep(innerPlan)
		steps = append(steps, step)
	}

	return exec.InParallel(steps, plan.InParallel.Limit, plan.InParallel.FailFast)
}

func (pb *planBuilder) buildAcrossStep(plan atc.Plan) exec.Step {
	stepMetadata := pb.stepMetadata(false)

	acrossStep := exec.Across(
		*plan.Across,
		pb.buildDelegateFactory(plan),
		stepMetadata,
	)

	return exec.LogError(acrossStep, pb.buildDelegateFactory(plan))
}

func (pb *planBuilder) buildDoStep(plan atc.Plan) exec.Step {
	var step exec.Step = exec.IdentityStep{}

	for i := len(*plan.Do) - 1; i >= 0; i-- {
		innerPlan := (*plan.Do)[i]
		innerPlan.Attempts = plan.Attempts
		previous := pb.buildStep(innerPlan)
		step = exec.OnSuccess(previous, step)
	}

	return step
}

func (pb *planBuilder) buildTimeoutStep(plan atc.Plan) exec.Step {
	innerPlan := plan.Timeout.Step
	innerPlan.Attempts = plan.Attempts
	step := pb.buildStep(innerPlan)
	return exec.Timeout(step, plan.Timeout.Duration)
}

func (pb *planBuilder) buildTryStep(plan atc.Plan) exec.Step {
	innerPlan := plan.Try.Step
	innerPlan.Attempts = plan.Attempts
	step := pb.buildStep(innerPlan)
	return exec.Try(step)
}

func (pb *planBuilder) buildOnAbortStep(plan atc.Plan) exec.Step {
	plan.OnAbort.Step.Attempts = plan.Attempts
	step := pb.buildStep(plan.OnAbort.Step)
	plan.OnAbort.Next.Attempts = plan.Attempts
	next := pb.withBuildStatus(db.BuildStatusAborted.String()).buildStep(plan.OnAbort.Next)
	return exec.OnAbort(step, next)
}

func (pb *planBuilder) buildOnErrorStep(plan atc.Plan) exec.Step {
	plan.OnError.Step.Attempts = plan.Attempts
	step := pb.buildStep(plan.OnError.Step)
	plan.OnError.Next.Attempts = plan.Attempts
	next := pb.withBuildStatus(db.BuildStatusErrored.String()).buildStep(plan.OnError.Next)
	return exec.OnError(step, next)
}

func (pb *planBuilder) buildOnSuccessStep(plan atc.Plan) exec.Step {
	plan.OnSuccess.Step.Attempts = plan.Attempts
	step := pb.buildStep(plan.OnSuccess.Step)
	plan.OnSuccess.Next.Attempts = plan.Attempts
	next := pb.withBuildStatus(db.BuildStatusSucceeded.String()).buildStep(plan.OnSuccess.Next)
	return exec.OnSuccess(step, next)
}

func (pb *planBuilder) buildOnFailureStep(plan atc.Plan) exec.Step {
	plan.OnFailure.Step.Attempts = plan.Attempts
	step := pb.buildStep(plan.OnFailure.Step)
	plan.OnFailure.Next.Attempts = plan.Attempts
	next := pb.withBuildStatus(db.BuildStatusFailed.String()).buildStep(plan.OnFailure.Next)
	return exec.OnFailure(step, next)
}

func (pb *planBuilder) buildEnsureStep(plan atc.Plan) exec.Step {
	plan.Ensure.Step.Attempts = plan.Attempts
	step := pb.buildStep(plan.Ensure.Step)
	plan.Ensure.Next.Attempts = plan.Attempts
	next := pb.buildStep(plan.Ensure.Next)
	return exec.Ensure(step, next)
}

func (pb *planBuilder) buildRetryStep(plan atc.Plan) exec.Step {
	steps := []exec.Step{}

	for index, innerPlan := range *plan.Retry {
		innerPlan.Attempts = append(plan.Attempts, index+1)

		step := pb.buildStep(innerPlan)
		steps = append(steps, step)
	}

	return exec.Retry(steps...)
}

func (pb *planBuilder) buildGetStep(plan atc.Plan) exec.Step {

	containerMetadata := pb.containerMetadata(
		db.ContainerTypeGet,
		plan.Get.Name,
		plan.Attempts,
	)

	stepMetadata := pb.stepMetadata(false)

	return pb.factory.coreFactory.GetStep(
		plan,
		stepMetadata,
		containerMetadata,
		pb.buildDelegateFactory(plan),
	)
}

func (pb *planBuilder) buildPutStep(plan atc.Plan) exec.Step {

	containerMetadata := pb.containerMetadata(
		db.ContainerTypePut,
		plan.Put.Name,
		plan.Attempts,
	)

	stepMetadata := pb.stepMetadata(plan.Put.ExposeBuildCreatedBy)

	return pb.factory.coreFactory.PutStep(
		plan,
		stepMetadata,
		containerMetadata,
		pb.buildDelegateFactory(plan),
	)
}

func (pb *planBuilder) buildCheckStep(plan atc.Plan) exec.Step {
	containerMetadata := pb.containerMetadata(
		db.ContainerTypeCheck,
		plan.Check.Name,
		plan.Attempts,
	)

	stepMetadata := pb.stepMetadata(false)

	return pb.factory.coreFactory.CheckStep(
		plan,
		stepMetadata,
		containerMetadata,
		pb.buildDelegateFactory(plan),
	)
}

func (pb *planBuilder) buildRunStep(plan atc.Plan) exec.Step {
	containerMetadata := pb.containerMetadata(
		db.ContainerTypeRun,
		plan.Run.Message,
		plan.Attempts,
	)

	stepMetadata := pb.stepMetadata(false)

	return pb.factory.coreFactory.RunStep(
		plan,
		stepMetadata,
		containerMetadata,
		pb.buildDelegateFactory(plan),
	)
}

func (pb *planBuilder) buildTaskStep(plan atc.Plan) exec.Step {

	containerMetadata := pb.containerMetadata(
		db.ContainerTypeTask,
		plan.Task.Name,
		plan.Attempts,
	)

	stepMetadata := pb.stepMetadata(false)

	return pb.factory.coreFactory.TaskStep(
		plan,
		stepMetadata,
		containerMetadata,
		pb.buildDelegateFactory(plan),
	)
}

func (pb *planBuilder) buildSetPipelineStep(plan atc.Plan) exec.Step {

	stepMetadata := pb.stepMetadata(false)

	return pb.factory.coreFactory.SetPipelineStep(
		plan,
		stepMetadata,
		pb.buildDelegateFactory(plan),
	)
}

func (pb *planBuilder) buildLoadVarStep(plan atc.Plan) exec.Step {

	stepMetadata := pb.stepMetadata(false)

	return pb.factory.coreFactory.LoadVarStep(
		plan,
		stepMetadata,
		pb.buildDelegateFactory(plan),
	)
}

func (pb *planBuilder) buildArtifactInputStep(plan atc.Plan) exec.Step {
	return pb.factory.coreFactory.ArtifactInputStep(
		plan,
		pb.build,
	)
}

func (pb *planBuilder) buildArtifactOutputStep(plan atc.Plan) exec.Step {
	return pb.factory.coreFactory.ArtifactOutputStep(
		plan,
		pb.build,
	)
}

func (pb *planBuilder) containerMetadata(
	containerType db.ContainerType,
	stepName string,
	attempts []int,
) db.ContainerMetadata {
	attemptStrs := []string{}
	for _, a := range attempts {
		attemptStrs = append(attemptStrs, strconv.Itoa(a))
	}

	var pipelineInstanceVars string
	if pb.build.PipelineInstanceVars() != nil {
		instanceVars, _ := json.Marshal(pb.build.PipelineInstanceVars())
		pipelineInstanceVars = string(instanceVars)
	}

	return db.ContainerMetadata{
		Type: containerType,

		PipelineID: pb.build.PipelineID(),
		JobID:      pb.build.JobID(),
		BuildID:    pb.build.ID(),

		PipelineName:         pb.build.PipelineName(),
		PipelineInstanceVars: pipelineInstanceVars,
		JobName:              pb.build.JobName(),
		BuildName:            pb.build.Name(),

		StepName: stepName,
		Attempt:  strings.Join(attemptStrs, "."),
	}
}

func (pb *planBuilder) stepMetadata(
	exposeBuildCreatedBy bool,
) exec.StepMetadata {
	meta := exec.StepMetadata{
		BuildID:              pb.build.ID(),
		BuildName:            pb.build.Name(),
		TeamID:               pb.build.TeamID(),
		TeamName:             pb.build.TeamName(),
		JobID:                pb.build.JobID(),
		JobName:              pb.build.JobName(),
		PipelineID:           pb.build.PipelineID(),
		PipelineName:         pb.build.PipelineName(),
		PipelineInstanceVars: pb.build.PipelineInstanceVars(),
		InstanceVarsQuery:    pb.build.PipelineRef().QueryParams(),
		ExternalURL:          pb.factory.externalURL,
		BuildStatus:          pb.buildStatus,
	}
	if exposeBuildCreatedBy && pb.build.CreatedBy() != nil {
		meta.CreatedBy = *pb.build.CreatedBy()
	}
	return meta
}
