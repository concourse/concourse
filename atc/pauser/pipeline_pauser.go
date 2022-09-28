package pauser

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
)

type pipelinePauser struct {
	daysSinceLastBuild           int
	daysSinceCreatedWithoutBuild int
	dbPipelinePauser             db.PipelinePauser
}

func NewPipelinePauser(dbPipelinePauser db.PipelinePauser, daysSinceLastBuild int, daysSinceCreatedWithoutBuild int) *pipelinePauser {
	return &pipelinePauser{
		daysSinceLastBuild:           daysSinceLastBuild,
		daysSinceCreatedWithoutBuild: daysSinceCreatedWithoutBuild,
		dbPipelinePauser:             dbPipelinePauser,
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
	err := p.dbPipelinePauser.PausePipelines(ctx, p.daysSinceLastBuild, p.daysSinceCreatedWithoutBuild)
	if err != nil {
		logger.Error("failed-to-pause-pipelines", err)
		return err
	}

	return nil
}
