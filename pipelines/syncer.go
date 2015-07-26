package pipelines

import (
	"os"

	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
)

//go:generate counterfeiter . PipelinesDB

type PipelinesDB interface {
	GetAllActivePipelines() ([]db.SavedPipeline, error)
}

type PipelineRunnerFactory func(db.PipelineDB) ifrit.Runner

type runningProcess struct {
	ifrit.Process

	Exited <-chan error
}

type Syncer struct {
	logger lager.Logger

	pipelinesDB           PipelinesDB
	pipelineDBFactory     db.PipelineDBFactory
	pipelineRunnerFactory PipelineRunnerFactory

	runningPipelines map[int]runningProcess
}

func NewSyncer(
	logger lager.Logger,
	pipelinesDB PipelinesDB,
	pipelineDBFactory db.PipelineDBFactory,
	pipelineRunnerFactory PipelineRunnerFactory,
) *Syncer {
	return &Syncer{
		logger:                logger,
		pipelinesDB:           pipelinesDB,
		pipelineDBFactory:     pipelineDBFactory,
		pipelineRunnerFactory: pipelineRunnerFactory,

		runningPipelines: map[int]runningProcess{},
	}
}

func (syncer *Syncer) Sync() {
	pipelines, err := syncer.pipelinesDB.GetAllActivePipelines()
	if err != nil {
		syncer.logger.Error("failed-to-get-active-pipelines", err)
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
			if pipeline.Paused {
				continue
			}

			if pipeline.ID == id {
				found = true
			}
		}

		if !found {
			syncer.logger.Debug("stopping-pipeline", lager.Data{"pipeline-id": id})
			runningPipeline.Process.Signal(os.Interrupt)
		}
	}

	for _, pipeline := range pipelines {
		if pipeline.Paused || syncer.isPipelineRunning(pipeline.ID) {
			continue
		}

		pipelineDB := syncer.pipelineDBFactory.Build(pipeline)
		runner := syncer.pipelineRunnerFactory(pipelineDB)

		syncer.logger.Debug("starting-pipeline", lager.Data{"pipeline": pipeline.Name})

		process := ifrit.Invoke(runner)

		syncer.runningPipelines[pipeline.ID] = runningProcess{
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
