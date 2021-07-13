package gc

import (
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"github.com/concourse/concourse/atc/db"
)

type pipelineCollector struct {
	pipelineLifecycle db.PipelineLifecycle
}

func NewPipelineCollector(pipelineLifecyle db.PipelineLifecycle) *pipelineCollector {
	return &pipelineCollector{
		pipelineLifecycle: pipelineLifecyle,
	}
}

func (pc *pipelineCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("pipeline-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	err := pc.pipelineLifecycle.ArchiveAbandonedPipelines()
	if err != nil {
		logger.Error("failed-to-automatically-archive-pipelines", err)
		return err
	}

	return nil
}
