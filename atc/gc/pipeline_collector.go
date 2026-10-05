package gc

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager/v3/lagerctx"
	"github.com/concourse/concourse/v8/atc/db"
)

type pipelineCollector struct {
	pipelineLifecycle db.PipelineLifecycle
	archivedExpiry    time.Duration
}

func NewPipelineCollector(pipelineLifecyle db.PipelineLifecycle, archivedExpiry time.Duration) *pipelineCollector {
	return &pipelineCollector{
		pipelineLifecycle: pipelineLifecyle,
		archivedExpiry:    archivedExpiry,
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

	err = pc.pipelineLifecycle.DestroyArchivedPipelines(pc.archivedExpiry)
	if err != nil {
		logger.Error("failed-to-destroy-archived-pipelines", err)
		return err
	}

	return nil
}
