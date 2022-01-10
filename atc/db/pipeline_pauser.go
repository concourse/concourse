package db

import (
	"context"
	"strconv"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

//counterfeiter:generate . PipelinePauser
type PipelinePauser interface {
	PausePipelines(ctx context.Context, daysSinceLastBuild int) error
}

func NewPipelinePauser(conn Conn, lockFactory lock.LockFactory) PipelinePauser {
	return &pipelinePauser{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

type pipelinePauser struct {
	conn        Conn
	lockFactory lock.LockFactory
}

func (p *pipelinePauser) PausePipelines(ctx context.Context, daysSinceLastBuild int) error {
	logger := lagerctx.FromContext(ctx).Session("pipeline-pauser")
	rows, err := pipelinesQuery.Where(sq.And{
		sq.Eq{
			"p.paused": false,
		},
		sq.Expr(`NOT EXISTS (SELECT 1 FROM jobs j
							JOIN builds b ON j.latest_completed_build_id = b.id
							WHERE j.pipeline_id = p.id
							AND b.end_time > CURRENT_DATE - ?::INTERVAL)`,
			strconv.Itoa(daysSinceLastBuild)+" day"),
	}).RunWith(p.conn).Query()

	if err != nil {
		return err
	}

	pipelines, err := scanPipelines(p.conn, p.lockFactory, rows)
	if err != nil {
		return err
	}

	for _, pipeline := range pipelines {
		err = pipeline.Pause("automatic-pipeline-pauser")
		loggingData := p.generateLoggingData(pipeline)
		if err != nil {
			logger.Error("failed-to-pause-pipeline", err, loggingData)
			return err
		}
		logger.Info("paused-pipeline", loggingData)
	}

	return nil
}

func (_ *pipelinePauser) generateLoggingData(pipeline Pipeline) lager.Data {
	loggingData := lager.Data{"pipeline": pipeline.Name(), "team": pipeline.TeamName()}
	if len(pipeline.InstanceVars()) > 0 {
		loggingData["instanceVars"] = pipeline.InstanceVars().String
	}

	return loggingData
}
