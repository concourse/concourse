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
	step := build.buildStep(logger, build.metadata.Plan, event.OriginLocation{0})
	source := step.Using(&exec.NoopArtifactSource{})

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

func (build *execBuild) buildStep(logger lager.Logger, plan atc.Plan, location event.OriginLocation) exec.Step {
	if plan.Aggregate != nil {
		logger = logger.Session("aggregate")

		step := exec.Aggregate{}

		var aID uint = 0
		for name, innerPlan := range *plan.Aggregate {
			step[name] = build.buildStep(logger.Session(name), innerPlan, location.Chain(aID))
			aID++
		}

		return step
	}

	if plan.Compose != nil {
		x := build.buildStep(logger, plan.Compose.A, location)
		y := build.buildStep(logger, plan.Compose.B, location.Incr(1))
		return exec.Compose(x, y)
	}

	if plan.Conditional != nil {
		logger = logger.Session("conditional", lager.Data{
			"on": plan.Conditional.Conditions,
		})

		return exec.Conditional{
			Conditions: plan.Conditional.Conditions,
			Step:       build.buildStep(logger, plan.Conditional.Plan, location),
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

		return build.factory.Task(
			build.taskIdentifier(plan.Task.Name, location),
			build.delegate.ExecutionDelegate(logger, *plan.Task, location),
			exec.Privileged(plan.Task.Privileged),
			configSource,
		)
	}

	if plan.Get != nil {
		logger = logger.Session("get", lager.Data{
			"name": plan.Get.Name,
		})

		return build.factory.Get(
			build.getIdentifier(plan.Get.Name, location),
			build.delegate.InputDelegate(logger, *plan.Get, location),
			atc.ResourceConfig{
				Name:   plan.Get.Resource,
				Type:   plan.Get.Type,
				Source: plan.Get.Source,
			},
			plan.Get.Params,
			plan.Get.Version,
		)
	}

	if plan.Put != nil {
		logger = logger.Session("put", lager.Data{
			"name": plan.Put.Resource,
		})

		return build.factory.Put(
			build.putIdentifier(plan.Put.Resource, location),
			build.delegate.OutputDelegate(logger, *plan.Put, location),
			atc.ResourceConfig{
				Name:   plan.Put.Resource,
				Type:   plan.Put.Type,
				Source: plan.Put.Source,
			},
			plan.Put.Params,
		)
	}

	return exec.Identity{}
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
