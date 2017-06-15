package engine

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"
	"github.com/tedsuo/ifrit"
)

type execMetadata struct {
	Plan atc.Plan
}

const execEngineName = "exec.v2"

type execEngine struct {
	factory         exec.Factory
	delegateFactory BuildDelegateFactory
	externalURL     string
	releaseCh       chan struct{}
}

func NewExecEngine(
	factory exec.Factory,
	delegateFactory BuildDelegateFactory,
	externalURL string,
) Engine {
	return &execEngine{
		factory:         factory,
		delegateFactory: delegateFactory,
		externalURL:     externalURL,
		releaseCh:       make(chan struct{}),
	}
}

func (engine *execEngine) Name() string {
	return execEngineName
}

func (engine *execEngine) CreateBuild(logger lager.Logger, build db.Build, plan atc.Plan) (Build, error) {
	return &execBuild{
		teamName:     build.TeamName(),
		teamID:       build.TeamID(),
		pipelineID:   build.PipelineID(),
		pipelineName: build.PipelineName(),
		jobID:        build.JobID(),
		jobName:      build.JobName(),
		buildID:      build.ID(),
		buildName:    build.Name(),

		stepMetadata: buildMetadata(build, engine.externalURL),

		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(build),
		metadata: execMetadata{
			Plan: plan,
		},

		releaseCh: engine.releaseCh,
		signals:   make(chan os.Signal, 1),
	}, nil
}

func (engine *execEngine) LookupBuild(logger lager.Logger, build db.Build) (Build, error) {
	var metadata execMetadata
	err := json.Unmarshal([]byte(build.EngineMetadata()), &metadata)
	if err != nil {
		logger.Error("invalid-metadata", err)
		return nil, err
	}

	return &execBuild{
		teamName:     build.TeamName(),
		teamID:       build.TeamID(),
		pipelineID:   build.PipelineID(),
		pipelineName: build.PipelineName(),
		jobID:        build.JobID(),
		jobName:      build.JobName(),
		buildID:      build.ID(),
		buildName:    build.Name(),

		stepMetadata: buildMetadata(build, engine.externalURL),

		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(build),
		metadata: metadata,

		releaseCh: engine.releaseCh,
		signals:   make(chan os.Signal, 1),
	}, nil
}

func (engine *execEngine) ReleaseAll(logger lager.Logger) {
	logger.Info("calling-release-in-exec-engine")
	close(engine.releaseCh)
}

func buildMetadata(build db.Build, externalURL string) StepMetadata {
	return StepMetadata{
		BuildID:      build.ID(),
		BuildName:    build.Name(),
		JobName:      build.JobName(),
		PipelineName: build.PipelineName(),
		TeamName:     build.TeamName(),
		ExternalURL:  externalURL,
	}
}

type execBuild struct {
	teamID       int
	teamName     string
	pipelineID   int
	pipelineName string
	jobID        int
	jobName      string
	buildID      int
	buildName    string

	stepMetadata StepMetadata

	factory  exec.Factory
	delegate BuildDelegate

	signals   chan os.Signal
	releaseCh chan struct{}

	metadata execMetadata
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
	source := stepFactory.Using(&exec.NoopStep{}, worker.NewArtifactRepository())

	process := ifrit.Background(source)

	exited := process.Wait()

	aborted := false
	var succeeded exec.Success

	for {
		select {
		case <-build.releaseCh:
			logger.Info("releasing")
			return
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

func (build *execBuild) workerMetadata(
	containerType db.ContainerType,
	stepName string,
	attempts []int,
) db.ContainerMetadata {
	attemptStrs := []string{}
	for _, a := range attempts {
		attemptStrs = append(attemptStrs, strconv.Itoa(a))
	}

	return db.ContainerMetadata{
		Type: containerType,

		PipelineID: build.pipelineID,
		JobID:      build.jobID,
		BuildID:    build.buildID,

		PipelineName: build.pipelineName,
		JobName:      build.jobName,
		BuildName:    build.buildName,

		StepName: stepName,
		Attempt:  strings.Join(attemptStrs, "."),
	}
}
