package pipelines

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/tedsuo/ifrit"
)

type PipelineRunnerFactory func(db.Pipeline) ifrit.Runner

type Syncer struct {
	logger lager.Logger

	pipelineFactory       db.PipelineFactory
	pipelineRunnerFactory PipelineRunnerFactory

	runningPipelines map[int]runningPipeline
}

type runningPipeline struct {
	Name string

	ifrit.Process

	Exited <-chan error
}

func NewSyncer(
	logger lager.Logger,
	pipelineFactory db.PipelineFactory,
	pipelineRunnerFactory PipelineRunnerFactory,
) *Syncer {
	return &Syncer{
		logger:                logger,
		pipelineFactory:       pipelineFactory,
		pipelineRunnerFactory: pipelineRunnerFactory,

		runningPipelines: map[int]runningPipeline{},
	}
}

func (syncer *Syncer) Sync() {
	pipelines, err := syncer.pipelineFactory.AllPipelines()
	if err != nil {
		syncer.logger.Error("failed-to-get-pipelines", err)
		return
	}

	for id, runningPipeline := range syncer.runningPipelines {
		select {
		case <-runningPipeline.Exited:
			syncer.logger.Debug("pipeline-exited", lager.Data{"pipeline-id": id})
			syncer.removePipeline(id)
		default:
		}

		var found bool
		for _, pipeline := range pipelines {
			if pipeline.Paused() {
				continue
			}

			if pipeline.ID() == id && pipeline.Name() == runningPipeline.Name {
				found = true
			}
		}

		if !found {
			syncer.logger.Debug("stopping-pipeline", lager.Data{"pipeline-id": id})
			runningPipeline.Process.Signal(os.Interrupt)
			syncer.removePipeline(id)
		}
	}

	for _, pipeline := range pipelines {
		if pipeline.Paused() || syncer.isPipelineRunning(pipeline.ID()) {
			continue
		}

		runner := syncer.pipelineRunnerFactory(pipeline)

		syncer.logger.Debug("starting-pipeline", lager.Data{"pipeline": pipeline.Name()})

		process := ifrit.Invoke(runner)

		syncer.runningPipelines[pipeline.ID()] = runningPipeline{
			Name:    pipeline.Name(),
			Process: process,
			Exited:  process.Wait(),
		}
	}
}

func (syncer *Syncer) removePipeline(pipelineID int) {
	delete(syncer.runningPipelines, pipelineID)
}

func (syncer *Syncer) isPipelineRunning(pipelineID int) bool {
	_, found := syncer.runningPipelines[pipelineID]
	return found
}
