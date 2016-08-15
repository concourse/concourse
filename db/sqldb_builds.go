package db

import (
	"database/sql"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
)

func (db *SQLDB) FindJobIDForBuild(buildID int) (int, bool, error) {
	row := db.conn.QueryRow(`
		SELECT j.id
		FROM jobs j
		LEFT OUTER JOIN builds b ON j.id = b.job_id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE b.id = $1
		`, buildID)
	var id int
	err := row.Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	return id, true, nil
}

func (db *SQLDB) GetBuildByID(buildID int) (Build, bool, error) {
	return db.buildFactory.ScanBuild(db.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		LEFT OUTER JOIN jobs j ON b.job_id = j.id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
		LEFT OUTER JOIN teams t ON b.team_id = t.id
		WHERE b.id = $1
	`, buildID))
}

func (db *SQLDB) GetPublicBuilds(page Page) ([]Build, Pagination, error) {
	buildsQuery := sq.Select(qualifiedBuildColumns).From("builds b").
		LeftJoin("jobs j ON b.job_id = j.id").
		LeftJoin("pipelines p ON j.pipeline_id = p.id").
		LeftJoin("teams t ON b.team_id = t.id").
		Where(sq.Eq{"p.public": true})

	return getBuildsWithPagination(buildsQuery, page, db.conn, db.buildFactory)
}

func (db *SQLDB) GetAllStartedBuilds() ([]Build, error) {
	rows, err := db.conn.Query(`
		SELECT ` + qualifiedBuildColumns + `
		FROM builds b
		LEFT OUTER JOIN jobs j ON b.job_id = j.id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
		LEFT OUTER JOIN teams t ON b.team_id = t.id
		WHERE b.status = 'started'
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	bs := []Build{}

	for rows.Next() {
		build, _, err := db.buildFactory.ScanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (db *SQLDB) DeleteBuildEventsByBuildIDs(buildIDs []int) error {
	if len(buildIDs) == 0 {
		return nil
	}

	interfaceBuildIDs := make([]interface{}, len(buildIDs))
	for i, buildID := range buildIDs {
		interfaceBuildIDs[i] = buildID
	}

	indexStrings := make([]string, len(buildIDs))
	for i := range indexStrings {
		indexStrings[i] = "$" + strconv.Itoa(i+1)
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(`
   DELETE FROM build_events
	 WHERE build_id IN (`+strings.Join(indexStrings, ",")+`)
	 `, interfaceBuildIDs...)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE builds
		SET reap_time = now()
		WHERE id IN (`+strings.Join(indexStrings, ",")+`)
	`, interfaceBuildIDs...)
	if err != nil {
		return err
	}

	err = tx.Commit()
	return err
}

func (db *SQLDB) FindLatestSuccessfulBuildsPerJob() (map[int]int, error) {
	rows, err := db.conn.Query(
		`SELECT max(id), job_id
		FROM builds
		WHERE job_id is not null
		AND status = 'succeeded'
		GROUP BY job_id`)

	if err != nil {
		if err == sql.ErrNoRows {
			return map[int]int{}, nil
		}
		return nil, err
	}

	latestSuccessfulBuildsPerJob := map[int]int{}
	for rows.Next() {
		var id, job_id int
		err := rows.Scan(&id, &job_id)
		if err != nil {
			if err == sql.ErrNoRows {
				return map[int]int{}, nil
			}
			return nil, err
		}

		latestSuccessfulBuildsPerJob[job_id] = id
	}

	return latestSuccessfulBuildsPerJob, nil
}

func getBuildsWithPagination(buildsQuery sq.SelectBuilder, page Page, dbConn Conn, buildFactory *buildFactory) ([]Build, Pagination, error) {
	var rows *sql.Rows
	var err error

	if page.Since == 0 && page.Until == 0 {
		buildsQuery = buildsQuery.OrderBy("b.id DESC").Limit(uint64(page.Limit))
	} else if page.Until != 0 {
		buildsQuery = buildsQuery.Where(sq.Gt{"b.id": uint64(page.Until)}).OrderBy("b.id ASC").Limit(uint64(page.Limit))
		buildsQuery = sq.Select("sub.*").FromSelect(buildsQuery, "sub").OrderBy("sub.id DESC")
	} else {
		buildsQuery = buildsQuery.Where(sq.Lt{"b.id": page.Since}).OrderBy("b.id DESC").Limit(uint64(page.Limit))
	}

	query, args, err := buildsQuery.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, Pagination{}, err
	}

	rows, err = dbConn.Query(query, args...)
	if err != nil {
		return nil, Pagination{}, err
	}

	defer rows.Close()

	builds := []Build{}

	for rows.Next() {
		build, _, err := buildFactory.ScanBuild(rows)
		if err != nil {
			return nil, Pagination{}, err
		}

		builds = append(builds, build)
	}

	if len(builds) == 0 {
		return builds, Pagination{}, nil
	}

	var minID int
	var maxID int

	maxMinBuildIDQuery, _, err := sq.Select(
		"COALESCE(MAX(id), 0) as maxID",
		"COALESCE(MIN(id), 0) as minID",
	).From("builds").ToSql()
	if err != nil {
		return nil, Pagination{}, err
	}

	err = dbConn.QueryRow(maxMinBuildIDQuery).Scan(&maxID, &minID)
	if err != nil {
		return nil, Pagination{}, err
	}

	first := builds[0]
	last := builds[len(builds)-1]

	var pagination Pagination

	if first.ID() < maxID {
		pagination.Previous = &Page{
			Until: first.ID(),
			Limit: page.Limit,
		}
	}

	if last.ID() > minID {
		pagination.Next = &Page{
			Since: last.ID(),
			Limit: page.Limit,
		}
	}

	return builds, pagination, nil
}
