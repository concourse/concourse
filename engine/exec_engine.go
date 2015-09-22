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
		buildID:      model.ID,
		stepMetadata: buildMetadata(model),

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
		buildID:      model.ID,
		stepMetadata: buildMetadata(model),

		db:       engine.db,
		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(model.ID),
		metadata: metadata,

		signals: make(chan os.Signal, 1),
	}, nil
}

func buildMetadata(model db.Build) StepMetadata {
	return StepMetadata{
		BuildID:      model.ID,
		BuildName:    model.Name,
		JobName:      model.JobName,
		PipelineName: model.PipelineName,
	}
}

type execBuild struct {
	buildID      int
	stepMetadata StepMetadata

	db EngineDB

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
		return build.buildAggregateStep(logger, plan)
	}

	if plan.Timeout != nil {
		return build.buildTimeoutStep(logger, plan)
	}

	if plan.Try != nil {
		return build.buildTryStep(logger, plan)
	}

	if plan.OnSuccess != nil {
		return build.buildOnSuccessStep(logger, plan)
	}

	if plan.OnFailure != nil {
		return build.buildOnFailureStep(logger, plan)
	}

	if plan.Ensure != nil {
		return build.buildEnsureStep(logger, plan)
	}

	if plan.Task != nil {
		return build.buildTaskStep(logger, plan)
	}

	if plan.Get != nil {
		return build.buildGetStep(logger, plan)
	}

	if plan.Put != nil {
		return build.buildPutStep(logger, plan)
	}

	if plan.DependentGet != nil {
		return build.buildDependentGetStep(logger, plan)
	}

	return exec.Identity{}
}

func (build *execBuild) taskIdentifier(name string, location event.OriginLocation) worker.Identifier {
	return worker.Identifier{
		BuildID:      build.buildID,
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
		BuildID:      build.buildID,
		Type:         "put",
		Name:         name,
		StepLocation: location.ID,
	}
}
