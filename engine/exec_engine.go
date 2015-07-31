package engine

import (
	"encoding/json"
	"errors"
	"strings"

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
	stepFactory := build.buildStepFactory(logger, build.metadata.Plan)
	source := stepFactory.Using(&exec.NoopStep{}, exec.NewSourceRepository())

	defer source.Release()

	process := ifrit.Background(source)

	exited := process.Wait()

	aborted := false
	var succeeded exec.Success

	for {
		select {
		case err := <-exited:
			if aborted || (err != nil && strings.Contains(err.Error(), exec.ErrStepTimedOut.Error())) {
				succeeded = false
			} else {
				if !source.Result(&succeeded) {
					logger.Error("step-had-no-result", errors.New("step failed to provide us with a result"))
					succeeded = false
				}
			}

			build.delegate.Finish(logger.Session("finish"), err, succeeded, aborted)
			return

		case sig := <-build.signals:
			process.Signal(sig)

			if sig == os.Kill {
				aborted = true
			}
		}
	}
}

func (build *execBuild) buildStepFactory(logger lager.Logger, plan atc.Plan) exec.StepFactory {
	if plan.Aggregate != nil {

		logger = logger.Session("aggregate")

		step := exec.Aggregate{}

		for _, innerPlan := range *plan.Aggregate {
			stepFactory := build.buildStepFactory(logger, innerPlan)

			step = append(step, stepFactory)
		}

		return step
	}

	if plan.Timeout != nil {
		step := build.buildStepFactory(logger, plan.Timeout.Step)
		return exec.Timeout(step, plan.Timeout.Duration)
	}

	if plan.Try != nil {
		step := build.buildStepFactory(logger, plan.Try.Step)
		return exec.Try(step)
	}

	if plan.OnSuccess != nil {
		step := build.buildStepFactory(logger, plan.OnSuccess.Step)
		next := build.buildStepFactory(logger, plan.OnSuccess.Next)
		return exec.OnSuccess(step, next)
	}

	if plan.OnFailure != nil {
		step := build.buildStepFactory(logger, plan.OnFailure.Step)
		next := build.buildStepFactory(logger, plan.OnFailure.Next)
		return exec.OnFailure(step, next)
	}

	if plan.Ensure != nil {
		step := build.buildStepFactory(logger, plan.Ensure.Step)
		next := build.buildStepFactory(logger, plan.Ensure.Next)
		return exec.Ensure(step, next)
	}

	if plan.Compose != nil {
		x := build.buildStepFactory(logger, plan.Compose.A)
		y := build.buildStepFactory(logger, plan.Compose.B)
		return exec.Compose(x, y)
	}

	if plan.Conditional != nil {
		logger = logger.Session("conditional", lager.Data{
			"on": plan.Conditional.Conditions,
		})

		steps := build.buildStepFactory(logger, plan.Conditional.Plan)

		return exec.Conditional{
			Conditions:  plan.Conditional.Conditions,
			StepFactory: steps,
		}
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

	if plan.Get != nil {
		logger = logger.Session("get", lager.Data{
			"name": plan.Get.Name,
		})

		var location event.OriginLocation
		if plan.Location != nil {
			location = event.OriginLocationFrom(*plan.Location)
		}

		return build.factory.Get(
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

	if plan.Put != nil {
		logger = logger.Session("put", lager.Data{
			"name": plan.Put.Name,
		})

		var location event.OriginLocation
		if plan.Location != nil {
			location = event.OriginLocationFrom(*plan.Location)
		}

		return build.factory.Put(
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

	if plan.DependentGet != nil {
		logger = logger.Session("get", lager.Data{
			"name": plan.DependentGet.Name,
		})

		var location event.OriginLocation
		if plan.Location != nil {
			location = event.OriginLocationFrom(*plan.Location)
		}

		getPlan := plan.DependentGet.GetPlan()
		return build.factory.DependentGet(
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

	return exec.Identity{}
}

func (build *execBuild) taskIdentifier(name string, location event.OriginLocation) worker.Identifier {
	return worker.Identifier{
		BuildID: build.buildID,

		Type:         "task",
		Name:         name,
		StepLocation: location.ID,
	}
}

func (build *execBuild) getIdentifier(name string, location event.OriginLocation) worker.Identifier {
	return worker.Identifier{
		BuildID:      build.buildID,
		Type:         "get",
		Name:         name,
		StepLocation: location.ID,
	}
}

func (build *execBuild) putIdentifier(name string, location event.OriginLocation) worker.Identifier {
	return worker.Identifier{
		BuildID: build.buildID,

		Type:         "put",
		Name:         name,
		StepLocation: location.ID,
	}
}
