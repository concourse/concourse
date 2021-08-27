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

func (factory *stepperFactory) StepperForBuild(build db.Build) (exec.Stepper, error) {
	if build.Schema() != supportedSchema {
		return nil, errors.New("schema not supported")
	}

	return func(plan atc.Plan) exec.Step {
		return factory.buildStep(build, plan)
	}, nil
}

func (factory *stepperFactory) buildDelegateFactory(build db.Build, plan atc.Plan) DelegateFactory {
	return DelegateFactory{
		build:           build,
		plan:            plan,
		rateLimiter:     factory.rateLimiter,
		policyChecker:   factory.policyChecker,
		dbWorkerFactory: factory.dbWorkerFactory,
		lockFactory:     factory.lockFactory,
	}
}

func (factory *stepperFactory) buildStep(build db.Build, plan atc.Plan) exec.Step {
	if plan.InParallel != nil {
		return factory.buildParallelStep(build, plan)
	}

	if plan.Across != nil {
		return factory.buildAcrossStep(build, plan)
	}

	if plan.Do != nil {
		return factory.buildDoStep(build, plan)
	}

	if plan.Timeout != nil {
		return factory.buildTimeoutStep(build, plan)
	}

	if plan.Try != nil {
		return factory.buildTryStep(build, plan)
	}

	if plan.OnAbort != nil {
		return factory.buildOnAbortStep(build, plan)
	}

	if plan.OnError != nil {
		return factory.buildOnErrorStep(build, plan)
	}

	if plan.OnSuccess != nil {
		return factory.buildOnSuccessStep(build, plan)
	}

	if plan.OnFailure != nil {
		return factory.buildOnFailureStep(build, plan)
	}

	if plan.Ensure != nil {
		return factory.buildEnsureStep(build, plan)
	}

	if plan.Run != nil {
		return factory.buildRunStep(build, plan)
	}

	if plan.Task != nil {
		return factory.buildTaskStep(build, plan)
	}

	if plan.SetPipeline != nil {
		return factory.buildSetPipelineStep(build, plan)
	}

	if plan.LoadVar != nil {
		return factory.buildLoadVarStep(build, plan)
	}

	if plan.Check != nil {
		return factory.buildCheckStep(build, plan)
	}

	if plan.Get != nil {
		return factory.buildGetStep(build, plan)
	}

	if plan.Put != nil {
		return factory.buildPutStep(build, plan)
	}

	if plan.Retry != nil {
		return factory.buildRetryStep(build, plan)
	}

	if plan.ArtifactInput != nil {
		return factory.buildArtifactInputStep(build, plan)
	}

	if plan.ArtifactOutput != nil {
		return factory.buildArtifactOutputStep(build, plan)
	}

	return exec.IdentityStep{}
}

func (factory *stepperFactory) buildParallelStep(build db.Build, plan atc.Plan) exec.Step {

	var steps []exec.Step

	for _, innerPlan := range plan.InParallel.Steps {
		innerPlan.Attempts = plan.Attempts
		step := factory.buildStep(build, innerPlan)
		steps = append(steps, step)
	}

	return exec.InParallel(steps, plan.InParallel.Limit, plan.InParallel.FailFast)
}

func (factory *stepperFactory) buildAcrossStep(build db.Build, plan atc.Plan) exec.Step {
	stepMetadata := factory.stepMetadata(
		build,
		factory.externalURL,
		false,
	)

	acrossStep := exec.Across(
		*plan.Across,
		factory.buildDelegateFactory(build, plan),
		stepMetadata,
	)

	return exec.LogError(acrossStep, factory.buildDelegateFactory(build, plan))
}

func (factory *stepperFactory) buildDoStep(build db.Build, plan atc.Plan) exec.Step {
	var step exec.Step = exec.IdentityStep{}

	for i := len(*plan.Do) - 1; i >= 0; i-- {
		innerPlan := (*plan.Do)[i]
		innerPlan.Attempts = plan.Attempts
		previous := factory.buildStep(build, innerPlan)
		step = exec.OnSuccess(previous, step)
	}

	return step
}

func (factory *stepperFactory) buildTimeoutStep(build db.Build, plan atc.Plan) exec.Step {
	innerPlan := plan.Timeout.Step
	innerPlan.Attempts = plan.Attempts
	step := factory.buildStep(build, innerPlan)
	return exec.Timeout(step, plan.Timeout.Duration)
}

func (factory *stepperFactory) buildTryStep(build db.Build, plan atc.Plan) exec.Step {
	innerPlan := plan.Try.Step
	innerPlan.Attempts = plan.Attempts
	step := factory.buildStep(build, innerPlan)
	return exec.Try(step)
}

func (factory *stepperFactory) buildOnAbortStep(build db.Build, plan atc.Plan) exec.Step {
	plan.OnAbort.Step.Attempts = plan.Attempts
	step := factory.buildStep(build, plan.OnAbort.Step)
	plan.OnAbort.Next.Attempts = plan.Attempts
	next := factory.buildStep(build, plan.OnAbort.Next)
	return exec.OnAbort(step, next)
}

func (factory *stepperFactory) buildOnErrorStep(build db.Build, plan atc.Plan) exec.Step {
	plan.OnError.Step.Attempts = plan.Attempts
	step := factory.buildStep(build, plan.OnError.Step)
	plan.OnError.Next.Attempts = plan.Attempts
	next := factory.buildStep(build, plan.OnError.Next)
	return exec.OnError(step, next)
}

func (factory *stepperFactory) buildOnSuccessStep(build db.Build, plan atc.Plan) exec.Step {
	plan.OnSuccess.Step.Attempts = plan.Attempts
	step := factory.buildStep(build, plan.OnSuccess.Step)
	plan.OnSuccess.Next.Attempts = plan.Attempts
	next := factory.buildStep(build, plan.OnSuccess.Next)
	return exec.OnSuccess(step, next)
}

func (factory *stepperFactory) buildOnFailureStep(build db.Build, plan atc.Plan) exec.Step {
	plan.OnFailure.Step.Attempts = plan.Attempts
	step := factory.buildStep(build, plan.OnFailure.Step)
	plan.OnFailure.Next.Attempts = plan.Attempts
	next := factory.buildStep(build, plan.OnFailure.Next)
	return exec.OnFailure(step, next)
}

func (factory *stepperFactory) buildEnsureStep(build db.Build, plan atc.Plan) exec.Step {
	plan.Ensure.Step.Attempts = plan.Attempts
	step := factory.buildStep(build, plan.Ensure.Step)
	plan.Ensure.Next.Attempts = plan.Attempts
	next := factory.buildStep(build, plan.Ensure.Next)
	return exec.Ensure(step, next)
}

func (factory *stepperFactory) buildRetryStep(build db.Build, plan atc.Plan) exec.Step {
	steps := []exec.Step{}

	for index, innerPlan := range *plan.Retry {
		innerPlan.Attempts = append(plan.Attempts, index+1)

		step := factory.buildStep(build, innerPlan)
		steps = append(steps, step)
	}

	return exec.Retry(steps...)
}

func (factory *stepperFactory) buildGetStep(build db.Build, plan atc.Plan) exec.Step {

	containerMetadata := factory.containerMetadata(
		build,
		db.ContainerTypeGet,
		plan.Get.Name,
		plan.Attempts,
	)

	stepMetadata := factory.stepMetadata(
		build,
		factory.externalURL,
		false,
	)

	return factory.coreFactory.GetStep(
		plan,
		stepMetadata,
		containerMetadata,
		factory.buildDelegateFactory(build, plan),
	)
}

func (factory *stepperFactory) buildPutStep(build db.Build, plan atc.Plan) exec.Step {

	containerMetadata := factory.containerMetadata(
		build,
		db.ContainerTypePut,
		plan.Put.Name,
		plan.Attempts,
	)

	stepMetadata := factory.stepMetadata(
		build,
		factory.externalURL,
		plan.Put.ExposeBuildCreatedBy,
	)

	return factory.coreFactory.PutStep(
		plan,
		stepMetadata,
		containerMetadata,
		factory.buildDelegateFactory(build, plan),
	)
}

func (factory *stepperFactory) buildCheckStep(build db.Build, plan atc.Plan) exec.Step {
	containerMetadata := factory.containerMetadata(
		build,
		db.ContainerTypeCheck,
		plan.Check.Name,
		plan.Attempts,
	)

	stepMetadata := factory.stepMetadata(
		build,
		factory.externalURL,
		false,
	)

	return factory.coreFactory.CheckStep(
		plan,
		stepMetadata,
		containerMetadata,
		factory.buildDelegateFactory(build, plan),
	)
}

func (factory *stepperFactory) buildRunStep(build db.Build, plan atc.Plan) exec.Step {
	containerMetadata := factory.containerMetadata(
		build,
		db.ContainerTypeRun,
		plan.Run.Message,
		plan.Attempts,
	)

	stepMetadata := factory.stepMetadata(
		build,
		factory.externalURL,
		false,
	)

	return factory.coreFactory.RunStep(
		plan,
		stepMetadata,
		containerMetadata,
		factory.buildDelegateFactory(build, plan),
	)
}

func (factory *stepperFactory) buildTaskStep(build db.Build, plan atc.Plan) exec.Step {

	containerMetadata := factory.containerMetadata(
		build,
		db.ContainerTypeTask,
		plan.Task.Name,
		plan.Attempts,
	)

	stepMetadata := factory.stepMetadata(
		build,
		factory.externalURL,
		false,
	)

	return factory.coreFactory.TaskStep(
		plan,
		stepMetadata,
		containerMetadata,
		factory.buildDelegateFactory(build, plan),
	)
}

func (factory *stepperFactory) buildSetPipelineStep(build db.Build, plan atc.Plan) exec.Step {

	stepMetadata := factory.stepMetadata(
		build,
		factory.externalURL,
		false,
	)

	return factory.coreFactory.SetPipelineStep(
		plan,
		stepMetadata,
		factory.buildDelegateFactory(build, plan),
	)
}

func (factory *stepperFactory) buildLoadVarStep(build db.Build, plan atc.Plan) exec.Step {

	stepMetadata := factory.stepMetadata(
		build,
		factory.externalURL,
		false,
	)

	return factory.coreFactory.LoadVarStep(
		plan,
		stepMetadata,
		factory.buildDelegateFactory(build, plan),
	)
}

func (factory *stepperFactory) buildArtifactInputStep(build db.Build, plan atc.Plan) exec.Step {
	return factory.coreFactory.ArtifactInputStep(
		plan,
		build,
	)
}

func (factory *stepperFactory) buildArtifactOutputStep(build db.Build, plan atc.Plan) exec.Step {
	return factory.coreFactory.ArtifactOutputStep(
		plan,
		build,
	)
}

func (factory *stepperFactory) containerMetadata(
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

func (factory *stepperFactory) stepMetadata(
	build db.Build,
	externalURL string,
	exposeBuildCreatedBy bool,
) exec.StepMetadata {
	meta := exec.StepMetadata{
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
	if exposeBuildCreatedBy && build.CreatedBy() != nil {
		meta.CreatedBy = *build.CreatedBy()
	}
	return meta
}
