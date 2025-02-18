package db

import (
	"context"
	"strconv"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerctx"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

//counterfeiter:generate . PipelinePauser
type PipelinePauser interface {
	PausePipelines(ctx context.Context, daysSinceLastBuild int) error
}

func NewPipelinePauser(conn DbConn, lockFactory lock.LockFactory) PipelinePauser {
	return &pipelinePauser{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

type pipelinePauser struct {
	conn        DbConn
	lockFactory lock.LockFactory
}

func (p *pipelinePauser) PausePipelines(ctx context.Context, daysSinceLastBuild int) error {
	logger := lagerctx.FromContext(ctx).Session("pipeline-pauser")
	rows, err := pipelinesQuery.Where(sq.And{
		sq.Eq{
			"p.paused": false,
		},
		// subquery returns a list of pipelines who jobs ran WITHIN the range.
		// These are the pipelines that SHOULD NOT be paused which we use to
		// build our list of pipelines that SHOULD be paused
		sq.Expr(`p.id NOT IN (SELECT j.pipeline_id FROM jobs j
							LEFT JOIN builds b ON j.latest_completed_build_id = b.id
							WHERE j.pipeline_id = p.id
								AND j.active = true
								AND (
									(b.end_time > CURRENT_DATE - ?::INTERVAL)
									--Don't pause pipelines with builds currently running
									OR j.next_build_id IS NOT NULL
								)
						)`,
			strconv.Itoa(daysSinceLastBuild)+" day"),
		// Covers edge case where pipelines that were just set could be paused automatically
		// Only pauses the pipeline if it was last updated more than ${daysSinceLastBuild} days ago
		sq.Expr(`p.last_updated < CURRENT_DATE - ?::INTERVAL`, strconv.Itoa(daysSinceLastBuild)+" day"),
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
