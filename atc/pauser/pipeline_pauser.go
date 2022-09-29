package pauser

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
)

type pipelinePauser struct {
	daysSinceLastBuild int
	dbPipelinePauser   db.PipelinePauser
}

func NewPipelinePauser(dbPipelinePauser db.PipelinePauser, daysSinceLastBuild int) *pipelinePauser {
	return &pipelinePauser{
		daysSinceLastBuild: daysSinceLastBuild,
		dbPipelinePauser:   dbPipelinePauser,
	}
}
func (p *pipelinePauser) Run(ctx context.Context) error {
	if p.daysSinceLastBuild == 0 {
		return nil
	}

	logger := lagerctx.FromContext(ctx).Session("automatic-pipeline-pauser")
	logger.Debug("start")
	defer logger.Debug("done")

	ctx = lagerctx.NewContext(ctx, logger)
	err := p.dbPipelinePauser.PausePipelines(ctx, p.daysSinceLastBuild)
	if err != nil {
		logger.Error("failed-to-pause-pipelines", err)
		return err
	}

	return nil
}
