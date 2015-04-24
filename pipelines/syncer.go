package pipelines

import (
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

	runningPipelines map[string]runningProcess
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

		runningPipelines: map[string]runningProcess{},
	}
}

func (syncer *Syncer) Sync() {
	pipelines, err := syncer.pipelinesDB.GetAllActivePipelines()
	if err != nil {
		return
	}

	for name, runningPipeline := range syncer.runningPipelines {
		select {
		case <-runningPipeline.Exited:
			syncer.removePipeline(name)
		default:
		}
	}

	for _, pipeline := range pipelines {
		if syncer.isPipelineRunning(pipeline.Name) {
			continue
		}

		pipelineDB := syncer.pipelineDBFactory.Build(pipeline)
		runner := syncer.pipelineRunnerFactory(pipelineDB)

		process := ifrit.Invoke(runner)

		syncer.runningPipelines[pipeline.Name] = runningProcess{
			Process: process,
			Exited:  process.Wait(),
		}
	}
}

func (syncer *Syncer) removePipeline(pipelineName string) {
	delete(syncer.runningPipelines, pipelineName)
}

func (syncer *Syncer) isPipelineRunning(pipelineName string) bool {
	_, found := syncer.runningPipelines[pipelineName]
	return found
}
