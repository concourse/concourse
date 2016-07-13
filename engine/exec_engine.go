package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
const successTTL = 5 * time.Minute
const failureTTL = 5 * time.Minute

type execEngine struct {
	factory         exec.Factory
	delegateFactory BuildDelegateFactory
	teamDBFactory   db.TeamDBFactory
	externalURL     string
}

func NewExecEngine(
	factory exec.Factory,
	delegateFactory BuildDelegateFactory,
	teamDBFactory db.TeamDBFactory,
	externalURL string,
) Engine {
	return &execEngine{
		factory:         factory,
		delegateFactory: delegateFactory,
		teamDBFactory:   teamDBFactory,
		externalURL:     externalURL,
	}
}

func (engine *execEngine) Name() string {
	return execEngineName
}

func (engine *execEngine) CreateBuild(logger lager.Logger, build db.Build, plan atc.Plan) (Build, error) {
	return &execBuild{
		buildID:      build.ID(),
		stepMetadata: buildMetadata(build, engine.externalURL),

		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(build),
		metadata: execMetadata{
			Plan: plan,
		},

		signals: make(chan os.Signal, 1),

		containerSuccessTTL: successTTL,
		containerFailureTTL: failureTTL,
	}, nil
}

func (engine *execEngine) LookupBuild(logger lager.Logger, build db.Build) (Build, error) {
	var metadata execMetadata
	err := json.Unmarshal([]byte(build.EngineMetadata()), &metadata)
	if err != nil {
		logger.Error("invalid-metadata", err)
		return nil, err
	}

	err = atc.NewPlanTraversal(engine.convertPipelineNameToID(build.TeamName())).Traverse(&metadata.Plan)
	if err != nil {
		return nil, err
	}

	return &execBuild{
		buildID:      build.ID(),
		stepMetadata: buildMetadata(build, engine.externalURL),

		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(build),
		metadata: metadata,

		signals: make(chan os.Signal, 1),

		containerSuccessTTL: successTTL,
		containerFailureTTL: failureTTL,
	}, nil
}

func (engine *execEngine) convertPipelineNameToID(teamName string) func(plan *atc.Plan) error {
	teamDB := engine.teamDBFactory.GetTeamDB(teamName)
	return func(plan *atc.Plan) error {
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

			savedPipeline, err := teamDB.GetPipelineByName(*pipelineName)

			if err != nil {
				return err
			}

			*pipelineID = savedPipeline.ID
			*pipelineName = ""
		}

		return nil
	}
}

func buildMetadata(build db.Build, externalURL string) StepMetadata {
	return StepMetadata{
		BuildID:      build.ID(),
		BuildName:    build.Name(),
		JobName:      build.JobName(),
		PipelineName: build.PipelineName(),
		ExternalURL:  externalURL,
	}
}

type execBuild struct {
	buildID      int
	stepMetadata StepMetadata

	factory  exec.Factory
	delegate BuildDelegate

	signals chan os.Signal

	metadata execMetadata

	containerSuccessTTL time.Duration
	containerFailureTTL time.Duration
}

func (build *execBuild) Metadata() string {
	payload, err := json.Marshal(build.metadata)
	if err != nil {
		panic("failed to marshal build metadata: " + err.Error())
	}

	return string(payload)
}

func (build *execBuild) PublicPlan(lager.Logger) (atc.PublicBuildPlan, error) {
	return atc.PublicBuildPlan{
		Schema: execEngineName,
		Plan:   build.metadata.Plan.Public(),
	}, nil
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
