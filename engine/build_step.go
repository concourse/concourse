package engine

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
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

func (build *execBuild) buildTaskStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("task")

	var configSource exec.TaskConfigSource
	if plan.Task.Config != nil && plan.Task.ConfigPath != "" {
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

	workerID, workerMetadata := build.stepIdentifier(
		logger.Session("taskIdentifier"),
		plan.Task.Name,
		plan.ID,
		plan.Task.Pipeline,
		plan.Attempts,
		"task",
	)

	return build.factory.Task(
		logger,
		exec.SourceName(plan.Task.Name),
		workerID,
		workerMetadata,
		build.delegate.ExecutionDelegate(logger, *plan.Task, event.OriginID(plan.ID)),
		exec.Privileged(plan.Task.Privileged),
		plan.Task.Tags,
		configSource,
		plan.Task.ResourceTypes,
	)
}

func (build *execBuild) buildGetStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("get", lager.Data{
		"name": plan.Get.Name,
	})

	workerID, workerMetadata := build.stepIdentifier(
		logger.Session("stepIdentifier"),
		plan.Get.Name,
		plan.ID,
		plan.Get.Pipeline,
		plan.Attempts,
		"get",
	)

	return build.factory.Get(
		logger,
		build.stepMetadata,
		exec.SourceName(plan.Get.Name),
		workerID,
		workerMetadata,
		build.delegate.InputDelegate(logger, *plan.Get, event.OriginID(plan.ID)),
		atc.ResourceConfig{
			Name:   plan.Get.Resource,
			Type:   plan.Get.Type,
			Source: plan.Get.Source,
		},
		plan.Get.Tags,
		plan.Get.Params,
		plan.Get.Version,
		plan.Get.ResourceTypes,
	)
}

func (build *execBuild) buildPutStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("put", lager.Data{
		"name": plan.Put.Name,
	})

	workerID, workerMetadata := build.stepIdentifier(
		logger.Session("stepIdentifier"),
		plan.Put.Name,
		plan.ID,
		plan.Put.Pipeline,
		plan.Attempts,
		"put",
	)

	return build.factory.Put(
		logger,
		build.stepMetadata,
		workerID,
		workerMetadata,
		build.delegate.OutputDelegate(logger, *plan.Put, event.OriginID(plan.ID)),
		atc.ResourceConfig{
			Name:   plan.Put.Resource,
			Type:   plan.Put.Type,
			Source: plan.Put.Source,
		},
		plan.Put.Tags,
		plan.Put.Params,
		plan.Put.ResourceTypes,
	)
}

func (build *execBuild) buildDependentGetStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("get", lager.Data{
		"name": plan.DependentGet.Name,
	})

	getPlan := plan.DependentGet.GetPlan()
	workerID, workerMetadata := build.stepIdentifier(
		logger.Session("stepIdentifier"),
		getPlan.Name,
		plan.ID,
		plan.DependentGet.Pipeline,
		plan.Attempts,
		"get",
	)

	return build.factory.DependentGet(
		logger,
		build.stepMetadata,
		exec.SourceName(getPlan.Name),
		workerID,
		workerMetadata,
		build.delegate.InputDelegate(logger, getPlan, event.OriginID(plan.ID)),
		atc.ResourceConfig{
			Name:   getPlan.Resource,
			Type:   getPlan.Type,
			Source: getPlan.Source,
		},
		getPlan.Tags,
		getPlan.Params,
		getPlan.ResourceTypes,
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
