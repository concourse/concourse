package engine

import (
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"
)

func (build *execBuild) buildAggregateStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("aggregate")

	step := exec.Aggregate{}

	for _, innerPlan := range *plan.Aggregate {
		innerPlan.Attempts = plan.Attempts
		stepFactory := build.buildStepFactory(logger, innerPlan)
		step = append(step, stepFactory)
	}

	return step
}

func (build *execBuild) buildDoStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("do")

	var step exec.StepFactory

	step = exec.Identity{}

	for i := len(*plan.Do) - 1; i >= 0; i-- {
		innerPlan := (*plan.Do)[i]
		innerPlan.Attempts = plan.Attempts
		previous := build.buildStepFactory(logger, innerPlan)
		step = exec.OnSuccess(previous, step)
	}

	return step
}

func (build *execBuild) buildTimeoutStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	innerPlan := plan.Timeout.Step
	innerPlan.Attempts = plan.Attempts
	step := build.buildStepFactory(logger, innerPlan)
	return exec.Timeout(step, plan.Timeout.Duration, clock.NewClock())
}

func (build *execBuild) buildTryStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	innerPlan := plan.Try.Step
	innerPlan.Attempts = plan.Attempts
	step := build.buildStepFactory(logger, innerPlan)
	return exec.Try(step)
}

func (build *execBuild) buildOnSuccessStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	plan.OnSuccess.Step.Attempts = plan.Attempts
	step := build.buildStepFactory(logger, plan.OnSuccess.Step)
	plan.OnSuccess.Next.Attempts = plan.Attempts
	next := build.buildStepFactory(logger, plan.OnSuccess.Next)
	return exec.OnSuccess(step, next)
}

func (build *execBuild) buildOnFailureStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	plan.OnFailure.Step.Attempts = plan.Attempts
	step := build.buildStepFactory(logger, plan.OnFailure.Step)
	plan.OnFailure.Next.Attempts = plan.Attempts
	next := build.buildStepFactory(logger, plan.OnFailure.Next)
	return exec.OnFailure(step, next)
}

func (build *execBuild) buildEnsureStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	plan.Ensure.Step.Attempts = plan.Attempts
	step := build.buildStepFactory(logger, plan.Ensure.Step)
	plan.Ensure.Next.Attempts = plan.Attempts
	next := build.buildStepFactory(logger, plan.Ensure.Next)
	return exec.Ensure(step, next)
}

// needs rootfs
func (build *execBuild) buildTaskStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("task")

	var configSource exec.TaskConfigSource
	if plan.Task.ConfigPath != "" && (plan.Task.Config != nil || plan.Task.Params != nil) {
		configSource = exec.MergedConfigSource{
			A: exec.FileConfigSource{plan.Task.ConfigPath},
			B: exec.StaticConfigSource{*plan.Task},
		}
	} else if plan.Task.Config != nil {
		configSource = exec.StaticConfigSource{*plan.Task}
	} else if plan.Task.ConfigPath != "" {
		configSource = exec.FileConfigSource{plan.Task.ConfigPath}
	} else {
		return exec.Identity{}
	}

	configSource = exec.ValidatingConfigSource{configSource}

	workerMetadata := build.workerMetadata(
		db.ContainerTypeTask,
		plan.Task.Name,
		plan.Attempts,
	)

	clock := clock.NewClock()

	return build.factory.Task(
		logger,
		build.teamID,
		build.buildID,
		plan.ID,
		worker.ArtifactName(plan.Task.Name),
		workerMetadata,
		build.delegate.ExecutionDelegate(logger, *plan.Task, event.OriginID(plan.ID)),
		exec.Privileged(plan.Task.Privileged),
		plan.Task.Tags,
		configSource,
		plan.Task.VersionedResourceTypes,
		plan.Task.InputMapping,
		plan.Task.OutputMapping,
		plan.Task.ImageArtifactName,
		clock,
	)
}

// needs rootfs
func (build *execBuild) buildGetStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("get", lager.Data{
		"name": plan.Get.Name,
	})

	workerMetadata := build.workerMetadata(
		db.ContainerTypeGet,
		plan.Get.Name,
		plan.Attempts,
	)

	return build.factory.Get(
		logger,
		build.teamID,
		build.buildID,
		plan,
		build.stepMetadata,
		workerMetadata,
		build.delegate.InputDelegate(logger, *plan.Get, event.OriginID(plan.ID)),
	)
}

// needs rootfs
func (build *execBuild) buildPutStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("put", lager.Data{
		"name": plan.Put.Name,
	})

	workerMetadata := build.workerMetadata(
		db.ContainerTypePut,
		plan.Put.Name,
		plan.Attempts,
	)

	return build.factory.Put(
		logger,
		build.teamID,
		build.buildID,
		plan.ID,
		build.stepMetadata,
		workerMetadata,
		build.delegate.OutputDelegate(logger, *plan.Put, event.OriginID(plan.ID)),
		atc.ResourceConfig{
			Name:   plan.Put.Resource,
			Type:   plan.Put.Type,
			Source: plan.Put.Source,
		},
		plan.Put.Tags,
		plan.Put.Params,
		plan.Put.VersionedResourceTypes,
	)
}

// needs rootfs
func (build *execBuild) buildDependentGetStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("get", lager.Data{
		"name": plan.DependentGet.Name,
	})

	getPlan := plan.DependentGet.GetPlan()

	workerMetadata := build.workerMetadata(
		db.ContainerTypeGet,
		getPlan.Name,
		plan.Attempts,
	)

	return build.factory.DependentGet(
		logger,
		build.teamID,
		build.buildID,
		plan.ID,
		build.stepMetadata,
		worker.ArtifactName(getPlan.Name),
		workerMetadata,
		build.delegate.InputDelegate(logger, getPlan, event.OriginID(plan.ID)),
		atc.ResourceConfig{
			Name:   getPlan.Resource,
			Type:   getPlan.Type,
			Source: getPlan.Source,
		},
		getPlan.Tags,
		getPlan.Params,
		getPlan.VersionedResourceTypes,
	)
}

func (build *execBuild) buildRetryStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("retry")

	step := exec.Retry{}

	for index, innerPlan := range *plan.Retry {
		innerPlan.Attempts = append(plan.Attempts, index+1)

		stepFactory := build.buildStepFactory(logger, innerPlan)
		step = append(step, stepFactory)
	}

	return step
}
