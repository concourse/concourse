package db

import (
	"strconv"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

type PipelinePauser interface {
	PausePipelines(daysSinceLastBuild int) error
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

func (p *pipelinePauser) PausePipelines(daysSinceLastBuild int) error {
	activePipelines, err := getActivePipelines(p.conn, daysSinceLastBuild)
	if err != nil {
		return err
	}

	rows, err := pipelinesQuery.Where(sq.And{
		sq.Eq{
			"p.paused": false,
		},
		sq.NotEq{
			"p.id": activePipelines,
		},
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
		if err != nil {
			return err
		}
	}

	return nil
}

// Couldn't put a subquery inside a WHERE clause: https://github.com/Masterminds/squirrel/issues/258
// This is a workaround. I really tried to put this in as a subquery similar to
// how it's done in worker_lifecycle.go but the placeholder value for the days
// was never getting parsed correctly, even after figuring out that it also
// needs to be cast to INTERVAL.
// Maybe I was doing something stupid; there may be a way to get this in as a subquery
func getActivePipelines(conn Conn, days int) ([]int, error) {
	stmt, err := conn.Prepare(`SELECT p.id FROM pipelines AS p
							LEFT JOIN jobs AS j ON j.pipeline_id = p.id
							WHERE j.last_scheduled > CURRENT_DATE - $1::INTERVAL`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(strconv.Itoa(days) + " day")
	if err != nil {
		return nil, err
	}

	var pipelineIds []int
	for rows.Next() {
		var id int
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		pipelineIds = append(pipelineIds, id)
	}

	return pipelineIds, nil
}
