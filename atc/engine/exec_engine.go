package engine

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/exec"
)

type execMetadata struct {
	Plan atc.Plan
}

const execEngineName = "exec.v2"

type execEngine struct {
	factory         exec.Factory
	delegateFactory BuildDelegateFactory
	externalURL     string

	releaseCh     chan struct{}
	trackedStates *sync.Map
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

		releaseCh:     make(chan struct{}),
		trackedStates: new(sync.Map),
	}
}

func (engine *execEngine) Name() string {
	return execEngineName
}

func (engine *execEngine) CreateBuild(logger lager.Logger, build db.Build, plan atc.Plan) (Build, error) {
	ctx, cancel := context.WithCancel(context.Background())

	return &execBuild{
		dbBuild: build,

		stepMetadata: buildMetadata(build, engine.externalURL),

		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(build),
		metadata: execMetadata{
			Plan: plan,
		},

		ctx:    ctx,
		cancel: cancel,

		releaseCh:     engine.releaseCh,
		trackedStates: engine.trackedStates,
	}, nil
}

func (engine *execEngine) LookupBuild(logger lager.Logger, build db.Build) (Build, error) {
	ctx, cancel := context.WithCancel(context.Background())

	var metadata execMetadata
	err := json.Unmarshal([]byte(build.EngineMetadata()), &metadata)
	if err != nil {
		logger.Error("invalid-metadata", err)
		return nil, err
	}

	return &execBuild{
		dbBuild: build,

		stepMetadata: buildMetadata(build, engine.externalURL),

		factory:  engine.factory,
		delegate: engine.delegateFactory.Delegate(build),
		metadata: metadata,

		ctx:    ctx,
		cancel: cancel,

		releaseCh:     engine.releaseCh,
		trackedStates: engine.trackedStates,
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
	dbBuild      db.Build
	stepMetadata StepMetadata

	factory  exec.Factory
	delegate BuildDelegate

	ctx    context.Context
	cancel func()

	releaseCh     chan struct{}
	trackedStates *sync.Map

	metadata execMetadata
}

func (build *execBuild) Metadata() string {
	payload, err := json.Marshal(build.metadata)
	if err != nil {
		panic("failed to marshal build metadata: " + err.Error())
	}

	return string(payload)
}

func (build *execBuild) Abort(lager.Logger) error {
	build.cancel()
	return nil
}

func (build *execBuild) Resume(logger lager.Logger) {
	step := build.buildStep(logger, build.metadata.Plan)

	runCtx := lagerctx.NewContext(build.ctx, logger)

	state := build.runState()
	defer build.clearRunState()

	done := make(chan error, 1)
	go func() {
		done <- step.Run(runCtx, state)
	}()

	for {
		select {
		case <-build.releaseCh:
			logger.Info("releasing")
			return
		case err := <-done:
			build.delegate.Finish(logger.Session("finish"), err, step.Succeeded())
			return
		}
	}
}

func (build *execBuild) ReceiveInput(logger lager.Logger, plan atc.PlanID, stream io.ReadCloser) {
	build.runState().SendUserInput(plan, stream)
}

func (build *execBuild) SendOutput(logger lager.Logger, plan atc.PlanID, output io.Writer) {
	build.runState().ReadPlanOutput(plan, output)
}

func (build *execBuild) runState() exec.RunState {
	existingState, _ := build.trackedStates.LoadOrStore(build.dbBuild.ID(), exec.NewRunState())
	return existingState.(exec.RunState)
}

func (build *execBuild) clearRunState() {
	build.trackedStates.Delete(build.dbBuild.ID())
}

func (build *execBuild) buildStep(logger lager.Logger, plan atc.Plan) exec.Step {
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

	if plan.OnAbort != nil {
		return build.buildOnAbortStep(logger, plan)
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

	if plan.Retry != nil {
		return build.buildRetryStep(logger, plan)
	}

	if plan.UserArtifact != nil {
		return build.buildUserArtifactStep(logger, plan)
	}

	if plan.ArtifactOutput != nil {
		return build.buildArtifactOutputStep(logger, plan)
	}

	return exec.IdentityStep{}
}

func (build *execBuild) containerMetadata(
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

		PipelineID: build.dbBuild.PipelineID(),
		JobID:      build.dbBuild.JobID(),
		BuildID:    build.dbBuild.ID(),

		PipelineName: build.dbBuild.PipelineName(),
		JobName:      build.dbBuild.JobName(),
		BuildName:    build.dbBuild.Name(),

		StepName: stepName,
		Attempt:  strings.Join(attemptStrs, "."),
	}
}
