package algorithm

import (
	"database/sql"
	"sort"

	sq "github.com/Masterminds/squirrel"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type VersionsDB struct {
	Runner sq.Runner

	JobIDs             map[string]int
	ResourceIDs        map[string]int
	DisabledVersionIDs map[int]bool
}

func (db VersionsDB) IsVersionFirstOccurrence(versionID int, jobID int, inputName string) (bool, error) {
	var exists bool
	err := db.Runner.QueryRow(`
	  SELECT EXISTS (
	    SELECT 1
	    FROM build_resource_config_version_inputs i
	    JOIN builds b ON b.id = i.build_id AND b.job_id = $2
	    WHERE version_md5 = (
				SELECT version_md5
				FROM resource_config_versions
				WHERE id = $1
			)
	    AND i.name = $3
	  )
	`, versionID, jobID, inputName).Scan(&exists)
	if err != nil {
		return false, err
	}

	return !exists, nil
}

func (db VersionsDB) LatestVersionOfResource(resourceID int) (int, error) {
	var scopeID int
	err := psql.Select("resource_config_scope_id").
		From("resources").
		Where(sq.Eq{"id": resourceID}).
		RunWith(db.Runner).
		QueryRow().
		Scan(&scopeID)
	if err != nil {
		return 0, err
	}

	var versionID int
	err = psql.Select("v.id").
		From("resource_config_versions v").
		Where(sq.Eq{"v.resource_config_scope_id": scopeID}).
		Where(sq.Expr("v.version_md5 NOT IN (SELECT version_md5 FROM resource_disabled_versions WHERE resource_id = ?)", resourceID)).
		OrderBy("check_order DESC").
		RunWith(db.Runner).
		QueryRow().
		Scan(&versionID)
	if err != nil {
		return 0, err
	}

	return versionID, nil
}

func (db VersionsDB) SuccessfulBuilds(jobID int) ([]int, error) {
	var buildIDs []int
	rows, err := psql.Select("b.id").
		From("builds b").
		Where(sq.Eq{
			"b.job_id": jobID,
			"b.status": "succeeded",
		}).
		OrderBy("b.id DESC").
		RunWith(db.Runner).
		Query()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var id int
		err := rows.Scan(&id)
		if err != nil {
			return nil, err
		}

		buildIDs = append(buildIDs, id)
	}

	return buildIDs, nil
}

func (db VersionsDB) BuildOutputs(buildID int) ([]LegacyResourceVersion, error) {
	uniqOutputs := map[int]LegacyResourceVersion{}

	rows, err := psql.Select("r.id", "v.id", "v.check_order").
		From("build_resource_config_version_inputs i").
		Join("resources r ON r.id = i.resource_id").
		Join("resource_config_versions v ON v.resource_config_scope_id = r.resource_config_scope_id AND v.version_md5 = i.version_md5").
		Where(sq.Eq{"i.build_id": buildID}).
		OrderBy("v.check_order ASC").
		RunWith(db.Runner).
		Query()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var output LegacyResourceVersion
		err := rows.Scan(&output.ResourceID, &output.VersionID, &output.CheckOrder)
		if err != nil {
			return nil, err
		}

		uniqOutputs[output.ResourceID] = output
	}

	rows, err = psql.Select("r.id", "v.id", "v.check_order").
		From("build_resource_config_version_outputs o").
		Join("resources r ON r.id = o.resource_id").
		Join("resource_config_versions v ON v.resource_config_scope_id = r.resource_config_scope_id AND v.version_md5 = o.version_md5").
		Where(sq.Eq{"o.build_id": buildID}).
		OrderBy("v.check_order ASC").
		RunWith(db.Runner).
		Query()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var output LegacyResourceVersion
		err := rows.Scan(&output.ResourceID, &output.VersionID, &output.CheckOrder)
		if err != nil {
			return nil, err
		}

		uniqOutputs[output.ResourceID] = output
	}

	outputs := []LegacyResourceVersion{}

	for _, o := range uniqOutputs {
		outputs = append(outputs, o)
	}

	return outputs, nil
}

func (db VersionsDB) FindVersionOfResource(versionID int) (bool, error) {
	var exists bool
	err := db.Runner.QueryRow(`SELECT EXISTS ( SELECT 1 from resource_config_versions WHERE id = $1 )`, versionID).Scan(&exists)
	return exists, err
}

func (db VersionsDB) LatestBuildID(jobID int) (int, bool, error) {
	var buildID int
	err := psql.Select("b.id").
		From("builds b").
		Where(sq.Eq{
			"b.job_id":    jobID,
			"b.scheduled": true,
		}).
		OrderBy("b.id DESC").
		Limit(1).
		RunWith(db.Runner).
		QueryRow().
		Scan(&buildID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}

	return buildID, true, nil
}

func (db VersionsDB) NextEveryVersion(buildID int, resourceID int) (int, error) {
	var checkOrder int
	err := psql.Select("rcv.check_order").
		From("resource_config_versions rcv, resources r, build_resource_config_version_inputs i").
		Where(sq.Eq{
			"i.build_id": buildID,
			"r.id":       resourceID,
		}).
		Where(sq.Expr("r.resource_config_scope_id = rcv.resource_config_scope_id")).
		Where(sq.Expr("i.version_md5 = rcv.version_md5")).
		Where(sq.Expr("i.resource_id = r.id")).
		RunWith(db.Runner).
		QueryRow().
		Scan(&checkOrder)
	if err != nil {
		if err == sql.ErrNoRows {
			return db.LatestVersionOfResource(resourceID)
		}
		return 0, err
	}

	var nextVersionID int
	err = psql.Select("rcv.id").
		From("resource_config_versions rcv").
		Where(sq.Expr("rcv.version_md5 NOT IN (SELECT version_md5 FROM resource_disabled_versions WHERE resource_id = ?)", resourceID)).
		Where(sq.Gt{"rcv.check_order": checkOrder}).
		OrderBy("rcv.check_order ASC").
		Limit(1).
		RunWith(db.Runner).
		QueryRow().
		Scan(&nextVersionID)

	if nextVersionID != 0 {
		return nextVersionID, nil
	}

	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}

	err = psql.Select("rcv.id").
		From("resource_config_versions rcv").
		Where(sq.Expr("rcv.version_md5 NOT IN (SELECT version_md5 FROM resource_disabled_versions WHERE resource_id = ?)", resourceID)).
		Where(sq.LtOrEq{"rcv.check_order": checkOrder}).
		OrderBy("rcv.check_order DESC").
		Limit(1).
		RunWith(db.Runner).
		QueryRow().
		Scan(&nextVersionID)

	if err != nil {
		return 0, err
	}

	return nextVersionID, nil
}

func (db VersionsDB) LatestConstraintBuildID(buildID int, passedJobID int) (int, bool, error) {
	var latestBuildID int

	err := psql.Select("p.from_build_id").
		From("build_pipes p").
		Join("builds b ON b.id = p.from_build_id").
		Where(sq.Eq{
			"p.to_build_id": buildID,
			"b.job_id":      passedJobID,
		}).
		RunWith(db.Runner).
		QueryRow().
		Scan(&latestBuildID)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}

		return 0, false, err
	}

	return latestBuildID, true, nil
}

func (db VersionsDB) UnusedBuilds(buildID int, jobID int) ([]int, error) {
	var buildIDs []int
	rows, err := psql.Select("id").
		From("builds").
		Where(sq.And{
			sq.Gt{"id": buildID},
			sq.Eq{"job_id": jobID},
		}).
		OrderBy("id ASC").
		RunWith(db.Runner).
		Query()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var buildID int

		err = rows.Scan(&buildID)
		if err != nil {
			return nil, err
		}

		buildIDs = append(buildIDs, buildID)
	}

	rows, err = psql.Select("id").
		From("builds").
		Where(sq.And{
			sq.LtOrEq{"id": buildID},
			sq.Eq{"job_id": jobID},
		}).
		OrderBy("id DESC").
		RunWith(db.Runner).
		Query()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var buildID int

		err = rows.Scan(&buildID)
		if err != nil {
			return nil, err
		}

		buildIDs = append(buildIDs, buildID)
	}

	return buildIDs, nil
}

// Order passed jobs by whether or not the build pipes of the current job has a
// fromBuildID of the passed job. If there are multiple passed jobs that have a
// build pipe to the current job, then order them by the least number of
// builds. If there are jobs with the same number of builds, order
// alphabetically.
func (db VersionsDB) OrderPassedJobs(currentJobID int, jobs JobSet) ([]int, error) {
	latestBuildID, found, err := db.LatestBuildID(currentJobID)
	if err != nil {
		return nil, err
	}

	buildPipeJobs := make(map[int]bool)

	if found {
		rows, err := psql.Select("b.job_id").
			From("builds b").
			Join("build_pipes bp ON bp.from_build_id = b.id").
			Where(sq.Eq{"bp.to_build_id": latestBuildID}).
			RunWith(db.Runner).
			Query()
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var jobID int

			err = rows.Scan(&jobID)
			if err != nil {
				return nil, err
			}

			buildPipeJobs[jobID] = true
		}
	}

	jobToBuilds := map[int]int{}
	for job, _ := range jobs {
		var buildNum int
		err := psql.Select("COUNT(id)").
			From("builds").
			Where(sq.Eq{"job_id": job}).
			RunWith(db.Runner).
			QueryRow().
			Scan(&buildNum)
		if err != nil {
			return nil, err
		}

		jobToBuilds[job] = buildNum
	}

	type jobBuilds struct {
		jobID    int
		buildNum int
	}

	var orderedJobBuilds []jobBuilds
	for j, b := range jobToBuilds {
		orderedJobBuilds = append(orderedJobBuilds, jobBuilds{j, b})
	}

	sort.Slice(orderedJobBuilds, func(i, j int) bool {
		if buildPipeJobs[orderedJobBuilds[i].jobID] == buildPipeJobs[orderedJobBuilds[j].jobID] {
			if orderedJobBuilds[i].buildNum == orderedJobBuilds[j].buildNum {
				return orderedJobBuilds[i].jobID < orderedJobBuilds[j].jobID
			}
			return orderedJobBuilds[i].buildNum < orderedJobBuilds[j].buildNum
		}

		return buildPipeJobs[orderedJobBuilds[i].jobID]
	})

	orderedJobs := []int{}
	for _, jobBuild := range orderedJobBuilds {
		orderedJobs = append(orderedJobs, jobBuild.jobID)
	}

	return orderedJobs, nil
}
