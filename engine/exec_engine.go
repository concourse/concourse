package engine

import (
	"encoding/json"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
)

type execMetadata struct {
	Plan atc.Plan
}

type execEngine struct {
	factory         exec.Factory
	delegateFactory BuildDelegateFactory
	db              EngineDB
}

func NewExecEngine(factory exec.Factory, delegateFactory BuildDelegateFactory, db EngineDB) Engine {
	return &execEngine{
		factory:         factory,
		delegateFactory: delegateFactory,
		db:              db,
	}
}

func (engine *execEngine) Name() string {
	return "exec.v1"
}

func (engine *execEngine) CreateBuild(model db.Build, plan atc.Plan) (Build, error) {
	return &execBuild{
		buildID:  model.ID,
		db:       engine.db,
		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(model.ID),
		metadata: execMetadata{
			Plan: plan,
		},

		signals: make(chan os.Signal, 1),
	}, nil
}

func (engine *execEngine) LookupBuild(model db.Build) (Build, error) {
	var metadata execMetadata
	err := json.Unmarshal([]byte(model.EngineMetadata), &metadata)
	if err != nil {
		return nil, err
	}

	return &execBuild{
		buildID:  model.ID,
		db:       engine.db,
		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(model.ID),
		metadata: metadata,

		signals: make(chan os.Signal, 1),
	}, nil
}

type execBuild struct {
	buildID int
	db      EngineDB

	factory  exec.Factory
	delegate BuildDelegate

	signals chan os.Signal

	metadata execMetadata
}

func (build *execBuild) Metadata() string {
	payload, err := json.Marshal(build.metadata)
	if err != nil {
		panic("failed to marshal build metadata: " + err.Error())
	}

	return string(payload)
}

func (build *execBuild) Abort() error {
	build.signals <- os.Kill
	return nil
}

func (build *execBuild) Resume(logger lager.Logger) {
	stepFactory, _ := build.buildStepFactory(logger, build.metadata.Plan, event.OriginLocation{0})
	source := stepFactory.Using(&exec.NoopStep{}, exec.NewSourceRepository())

	defer source.Release()

	process := ifrit.Background(source)

	exited := process.Wait()

	for {
		select {
		case err := <-exited:
			build.delegate.Finish(logger.Session("finish"), err)
			return

		case sig := <-build.signals:
			process.Signal(sig)

			if sig == os.Kill {
				build.delegate.Aborted(logger)
			}
		}
	}
}

func (build *execBuild) buildStepFactory(logger lager.Logger, plan atc.Plan, location event.OriginLocation) (exec.StepFactory, event.OriginLocationIncrement) {
	if plan.Aggregate != nil {
		logger = logger.Session("aggregate")

		step := exec.Aggregate{}

		var aID event.OriginLocationIncrement = 0
		for _, innerPlan := range *plan.Aggregate {
			stepFactory, locationIncrement := build.buildStepFactory(logger, innerPlan, location.Chain(uint(aID)))
			step = append(step, stepFactory)
			aID = aID + locationIncrement
		}

		return step, event.SingleIncrement
	}

	if plan.HookedCompose != nil {
		step, stepIncrement := build.buildStepFactory(logger, plan.HookedCompose.Step, location)
		failure, failureIncrement := build.buildStepFactory(logger, plan.HookedCompose.OnFailure, location.Incr(stepIncrement))
		success, successIncrement := build.buildStepFactory(logger, plan.HookedCompose.OnSuccess, location.Incr(stepIncrement+failureIncrement))
		ensure, ensureIncrement := build.buildStepFactory(logger, plan.HookedCompose.OnCompletion, location.Incr(stepIncrement+failureIncrement+successIncrement))

		nextStep, nextStepIncrement := build.buildStepFactory(logger, plan.HookedCompose.Next, location.Incr(stepIncrement+failureIncrement+successIncrement+ensureIncrement))
		return exec.HookedCompose(step, nextStep, failure, success, ensure), stepIncrement + failureIncrement + successIncrement + ensureIncrement + nextStepIncrement
	}

	if plan.Compose != nil {
		x, xLocationIncrement := build.buildStepFactory(logger, plan.Compose.A, location)
		y, yLocationIncrement := build.buildStepFactory(logger, plan.Compose.B, location.Incr(xLocationIncrement))
		return exec.Compose(x, y), xLocationIncrement + yLocationIncrement
	}

	if plan.Conditional != nil {
		logger = logger.Session("conditional", lager.Data{
			"on": plan.Conditional.Conditions,
		})

		steps, locationIncrement := build.buildStepFactory(logger, plan.Conditional.Plan, location)

		return exec.Conditional{
			Conditions:  plan.Conditional.Conditions,
			StepFactory: steps,
		}, locationIncrement
	}

	if plan.Task != nil {
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
			return exec.Identity{}, event.NoIncrement
		}

		return build.factory.Task(
			exec.SourceName(plan.Task.Name),
			build.taskIdentifier(plan.Task.Name, location),
			build.delegate.ExecutionDelegate(logger, *plan.Task, location),
			exec.Privileged(plan.Task.Privileged),
			plan.Task.Tags,
			configSource,
		), event.SingleIncrement
	}

	if plan.Get != nil {
		logger = logger.Session("get", lager.Data{
			"name": plan.Get.Name,
		})

		return build.factory.Get(
			exec.SourceName(plan.Get.Name),
			build.getIdentifier(plan.Get.Name, location),
			build.delegate.InputDelegate(logger, *plan.Get, location, false),
			atc.ResourceConfig{
				Name:   plan.Get.Resource,
				Type:   plan.Get.Type,
				Source: plan.Get.Source,
			},
			plan.Get.Params,
			plan.Get.Tags,
			plan.Get.Version,
		), event.SingleIncrement
	}

	if plan.PutGet != nil {
		putPlan := plan.PutGet.Head.Put
		logger = logger.Session("put", lager.Data{
			"name": putPlan.Resource,
		})

		getPlan := putPlan.GetPlan()

		getLocation := location.Incr(1)
		restLocation := location.Incr(2)

		restOfSteps, restLocationIncrement := build.buildStepFactory(logger, plan.PutGet.Rest, restLocation)

		return exec.HookedCompose(
			build.factory.Put(
				build.putIdentifier(putPlan.Resource, location),
				build.delegate.OutputDelegate(logger, *putPlan, location),
				atc.ResourceConfig{
					Name:   putPlan.Resource,
					Type:   putPlan.Type,
					Source: putPlan.Source,
				},
				putPlan.Tags,
				putPlan.Params,
			),
			restOfSteps,
			exec.Identity{},
			build.factory.DependentGet(
				exec.SourceName(getPlan.Name),
				build.getIdentifier(getPlan.Name, getLocation),
				build.delegate.InputDelegate(logger, getPlan, getLocation, true),
				atc.ResourceConfig{
					Name:   getPlan.Resource,
					Type:   getPlan.Type,
					Source: getPlan.Source,
				},
				getPlan.Tags,
				getPlan.Params,
			),
			exec.Identity{},
		), event.OriginLocationIncrement(2) + restLocationIncrement
	}

	return exec.Identity{}, event.NoIncrement
}

func (build *execBuild) taskIdentifier(name string, location event.OriginLocation) worker.Identifier {
	return worker.Identifier{
		BuildID: build.buildID,

		Type:         "task",
		Name:         name,
		StepLocation: location,
	}
}

func (build *execBuild) getIdentifier(name string, location event.OriginLocation) worker.Identifier {
	return worker.Identifier{
		BuildID: build.buildID,

		Type:         "get",
		Name:         name,
		StepLocation: location,
	}
}

func (build *execBuild) putIdentifier(name string, location event.OriginLocation) worker.Identifier {
	return worker.Identifier{
		BuildID: build.buildID,

		Type:         "put",
		Name:         name,
		StepLocation: location,
	}
}
