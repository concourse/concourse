package dbng

import sq "github.com/Masterminds/squirrel"

type BuildFactory interface {
	MarkNonInterceptibleBuilds() error
}

type buildFactory struct {
	conn Conn
}

func NewBuildFactory(conn Conn) BuildFactory {
	return &buildFactory{
		conn: conn,
	}
}

func (f *buildFactory) MarkNonInterceptibleBuilds() error {
	latestBuildsPrefix := `WITH
		latest_builds AS (
			SELECT COALESCE(MAX(b.id)) AS build_id
			FROM builds b, jobs j
			WHERE b.job_id = j.id
			AND b.completed
			GROUP BY j.id
		)`

	_, err := psql.Update("builds").
		Prefix(latestBuildsPrefix).
		Set("interceptible", false).
		Where(sq.Or{
			sq.Expr("id NOT IN (select build_id FROM latest_builds)"),
			sq.And{
				sq.NotEq{"status": string(BuildStatusAborted)},
				sq.NotEq{"status": string(BuildStatusFailed)},
				sq.NotEq{"status": string(BuildStatusErrored)},
			},
		}).
		Where(sq.Eq{
			"completed": true,
		}).
		RunWith(f.conn).
		Exec()
	if err != nil {
		return err
	}

	return nil

}
