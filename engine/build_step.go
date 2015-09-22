package engine

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
	"github.com/pivotal-golang/lager"
)

func (build *execBuild) buildAggregateStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("aggregate")

	step := exec.Aggregate{}

	for _, innerPlan := range *plan.Aggregate {
		stepFactory := build.buildStepFactory(logger, innerPlan)
		step = append(step, stepFactory)
	}

	return step
}

func (build *execBuild) buildTimeoutStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	step := build.buildStepFactory(logger, plan.Timeout.Step)
	return exec.Timeout(step, plan.Timeout.Duration)
}

func (build *execBuild) buildTryStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	step := build.buildStepFactory(logger, plan.Try.Step)
	return exec.Try(step)
}

func (build *execBuild) buildOnSuccessStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	step := build.buildStepFactory(logger, plan.OnSuccess.Step)
	next := build.buildStepFactory(logger, plan.OnSuccess.Next)
	return exec.OnSuccess(step, next)
}

func (build *execBuild) buildOnFailureStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	step := build.buildStepFactory(logger, plan.OnFailure.Step)
	next := build.buildStepFactory(logger, plan.OnFailure.Next)
	return exec.OnFailure(step, next)
}

func (build *execBuild) buildEnsureStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	step := build.buildStepFactory(logger, plan.Ensure.Step)
	next := build.buildStepFactory(logger, plan.Ensure.Next)
	return exec.Ensure(step, next)
}

func (build *execBuild) buildTaskStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("task")

	var configSource exec.TaskConfigSource
	if plan.Task.Config != nil && plan.Task.ConfigPath != "" {
		configSource = exec.MergedConfigSource{
			A: exec.FileConfigSource{plan.Task.ConfigPath},
			B: exec.StaticConfigSource{*plan.Task.Config},
		}
	} else if plan.Task.Config != nil {
		configSource = exec.StaticConfigSource{*plan.Task.Config}
	} else if plan.Task.ConfigPath != "" {
		configSource = exec.FileConfigSource{plan.Task.ConfigPath}
	} else {
		return exec.Identity{}
	}

	var location event.OriginLocation
	if plan.Location != nil {
		location = event.OriginLocationFrom(*plan.Location)
	}

	return build.factory.Task(
		exec.SourceName(plan.Task.Name),
		build.taskIdentifier(plan.Task.Name, location),
		build.delegate.ExecutionDelegate(logger, *plan.Task, location),
		exec.Privileged(plan.Task.Privileged),
		plan.Task.Tags,
		configSource,
	)
}

func (build *execBuild) buildGetStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("get", lager.Data{
		"name": plan.Get.Name,
	})

	var location event.OriginLocation
	if plan.Location != nil {
		location = event.OriginLocationFrom(*plan.Location)
	}

	return build.factory.Get(
		build.stepMetadata,
		exec.SourceName(plan.Get.Name),
		build.getIdentifier(plan.Get.Name, location),
		build.delegate.InputDelegate(logger, *plan.Get, location),
		atc.ResourceConfig{
			Name:   plan.Get.Resource,
			Type:   plan.Get.Type,
			Source: plan.Get.Source,
		},
		plan.Get.Params,
		plan.Get.Tags,
		plan.Get.Version,
	)
}

func (build *execBuild) buildPutStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("put", lager.Data{
		"name": plan.Put.Name,
	})

	var location event.OriginLocation
	if plan.Location != nil {
		location = event.OriginLocationFrom(*plan.Location)
	}

	return build.factory.Put(
		build.stepMetadata,
		build.putIdentifier(plan.Put.Name, location),
		build.delegate.OutputDelegate(logger, *plan.Put, location),
		atc.ResourceConfig{
			Name:   plan.Put.Resource,
			Type:   plan.Put.Type,
			Source: plan.Put.Source,
		},
		plan.Put.Tags,
		plan.Put.Params,
	)
}

func (build *execBuild) buildDependentGetStep(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	logger = logger.Session("get", lager.Data{
		"name": plan.DependentGet.Name,
	})

	var location event.OriginLocation
	if plan.Location != nil {
		location = event.OriginLocationFrom(*plan.Location)
	}

	getPlan := plan.DependentGet.GetPlan()
	return build.factory.DependentGet(
		build.stepMetadata,
		exec.SourceName(getPlan.Name),
		build.getIdentifier(getPlan.Name, location),
		build.delegate.InputDelegate(logger, getPlan, location),
		atc.ResourceConfig{
			Name:   getPlan.Resource,
			Type:   getPlan.Type,
			Source: getPlan.Source,
		},
		getPlan.Tags,
		getPlan.Params,
	)
}
