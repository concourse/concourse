package engine

import (
	"encoding/json"
	"errors"
	"fmt"

	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
)

type execMetadata struct {
	Plan atc.Plan
}

const execEngineName = "exec.v2"

type execEngine struct {
	factory         exec.Factory
	delegateFactory BuildDelegateFactory
	db              EngineDB
	externalURL     string
}

func NewExecEngine(factory exec.Factory, delegateFactory BuildDelegateFactory, db EngineDB, externalURL string) Engine {
	return &execEngine{
		factory:         factory,
		delegateFactory: delegateFactory,
		db:              db,
		externalURL:     externalURL,
	}
}

func (engine *execEngine) Name() string {
	return execEngineName
}

func (engine *execEngine) CreateBuild(logger lager.Logger, model db.Build, plan atc.Plan) (Build, error) {
	return &execBuild{
		buildID:      model.ID,
		stepMetadata: buildMetadata(model, engine.externalURL),

		db:       engine.db,
		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(model.ID, model.PipelineID),
		metadata: execMetadata{
			Plan: plan,
		},

		signals: make(chan os.Signal, 1),
	}, nil
}

func (engine *execEngine) LookupBuild(logger lager.Logger, model db.Build) (Build, error) {
	var metadata execMetadata
	err := json.Unmarshal([]byte(model.EngineMetadata), &metadata)
	if err != nil {
		logger.Error("invalid-metadata", err)
		return nil, err
	}

	err = atc.NewPlanTraversal(engine.convertPipelineNameToID).Traverse(&metadata.Plan)
	if err != nil {
		return nil, err
	}

	return &execBuild{
		buildID:      model.ID,
		stepMetadata: buildMetadata(model, engine.externalURL),

		db:       engine.db,
		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(model.ID, model.PipelineID),
		metadata: metadata,

		signals: make(chan os.Signal, 1),
	}, nil
}

func (engine *execEngine) convertPipelineNameToID(plan *atc.Plan) error {
	var pipelineName *string
	var pipelineID *int

	switch {
	case plan.Get != nil:
		pipelineName = &plan.Get.Pipeline
		pipelineID = &plan.Get.PipelineID
	case plan.Put != nil:
		pipelineName = &plan.Put.Pipeline
		pipelineID = &plan.Put.PipelineID
	case plan.Task != nil:
		pipelineName = &plan.Task.Pipeline
		pipelineID = &plan.Task.PipelineID
	case plan.DependentGet != nil:
		pipelineName = &plan.DependentGet.Pipeline
		pipelineID = &plan.DependentGet.PipelineID
	}

	if pipelineName != nil && *pipelineName != "" {
		if *pipelineID != 0 {
			return fmt.Errorf(
				"build plan with ID %s has both pipeline name (%s) and ID (%d)",
				plan.ID,
				*pipelineName,
				*pipelineID,
			)
		}

		savedPipeline, err := engine.db.GetPipelineByTeamNameAndName(atc.DefaultTeamName, *pipelineName)

		if err != nil {
			return err
		}

		*pipelineID = savedPipeline.ID
		*pipelineName = ""
	}

	return nil
}

func buildMetadata(model db.Build, externalURL string) StepMetadata {
	return StepMetadata{
		BuildID:      model.ID,
		BuildName:    model.Name,
		JobName:      model.JobName,
		PipelineName: model.PipelineName,
		ExternalURL:  externalURL,
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

func (build *execBuild) PublicPlan(lager.Logger) (atc.PublicBuildPlan, bool, error) {
	return atc.PublicBuildPlan{
		Schema: execEngineName,
		Plan:   build.metadata.Plan.Public(),
	}, true, nil
}

func (build *execBuild) Abort(lager.Logger) error {
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
			if aborted {
				succeeded = false
			} else if !source.Result(&succeeded) {
				logger.Error("step-had-no-result", errors.New("step failed to provide us with a result"))
				succeeded = false
			}

			build.delegate.Finish(logger.Session("finish"), err, succeeded, aborted)
			//GetBuild
			dbBuild, found, err := build.db.GetBuild(build.buildID)
			println(dbBuild.ID)
			if err != nil {
				logger.Error("get-build", err)
			}

			if !found {
				logger.Error("build-not-found", errors.New("build not found"))
			}

			// latestFinishedBuild, found, err := build.db.GetLatestFinishedBuild(dbBuild.JobID)
			// if err != nil {
			// 	logger.Error("get-latest-finished-build", err)
			// }
			//
			// if !found {
			// 	logger.Error("latest-finished-build-not-found", errors.New("latest finished build not found"))
			// }

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

	if plan.Do != nil {
		return build.buildDoStep(logger, plan)
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

	if plan.Retry != nil {
		return build.buildRetryStep(logger, plan)
	}

	return exec.Identity{}
}

func (build *execBuild) stepIdentifier(
	logger lager.Logger,
	stepName string,
	planID atc.PlanID,
	pipelineID int,
	attempts []int,
	typ string,
) (worker.Identifier, worker.Metadata) {
	stepType, err := db.ContainerTypeFromString(typ)
	if err != nil {
		logger.Debug(fmt.Sprintf("Invalid step type: %s", typ))
	}

	return worker.Identifier{
			BuildID: build.buildID,
			PlanID:  planID,
		},
		worker.Metadata{
			StepName:   stepName,
			Type:       stepType,
			PipelineID: pipelineID,
			Attempts:   attempts,
		}
}
