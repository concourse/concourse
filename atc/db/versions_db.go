package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	gocache "github.com/patrickmn/go-cache"
)

const algorithmLimitRows = 100

type VersionsDB struct {
	Conn Conn

	Cache *gocache.Cache

	JobIDs           map[string]int
	ResourceIDs      map[string]int
	DisabledVersions map[int]map[string]bool
}

func (versions VersionsDB) VersionIsDisabled(resourceID int, versionMD5 ResourceVersion) bool {
	md5s, found := versions.DisabledVersions[resourceID]
	return found && md5s[string(versionMD5)]
}

func (versions VersionsDB) LatestVersionOfResource(resourceID int) (ResourceVersion, bool, error) {
	tx, err := versions.Conn.Begin()
	if err != nil {
		return "", false, err
	}

	defer tx.Rollback()

	version, found, err := versions.latestVersionOfResource(tx, resourceID)
	if err != nil {
		return "", false, err
	}

	if !found {
		return "", false, nil
	}

	err = tx.Commit()
	if err != nil {
		return "", false, err
	}

	return version, true, nil
}

func (versions VersionsDB) SuccessfulBuilds(jobID int, buildsIDCursor int) ([]int, int, error) {
	cacheKey := fmt.Sprintf("sb%d-%d", jobID, buildsIDCursor)

	var buildIDs []int
	c, found := versions.Cache.Get(cacheKey)
	if found {
		buildIDs = c.([]int)
	} else {
		builder := psql.Select("b.id").
			From("builds b").
			Where(sq.Eq{
				"b.job_id": jobID,
				"b.status": "succeeded",
			})

		if buildsIDCursor != 0 {
			builder = builder.Where(sq.Lt{"b.id": buildsIDCursor})
		}

		rows, err := builder.OrderBy("b.id DESC").
			RunWith(versions.Conn).
			Limit(algorithmLimitRows).
			Query()
		if err != nil {
			return nil, 0, err
		}

		for rows.Next() {
			var id int
			err := rows.Scan(&id)
			if err != nil {
				return nil, 0, err
			}

			buildIDs = append(buildIDs, id)
		}

		versions.Cache.Set(cacheKey, buildIDs, gocache.DefaultExpiration)
	}

	var newBuildIDCursor int
	if len(buildIDs) > 0 {
		newBuildIDCursor = buildIDs[len(buildIDs)-1]
	}

	return buildIDs, newBuildIDCursor, nil
}

func (versions VersionsDB) SuccessfulBuildsVersionConstrained(jobID int, version ResourceVersion, resourceID int, buildsIDCursor int) ([]int, int, error) {
	cacheKey := fmt.Sprintf("sbvc%d-%s-%d", jobID, version, buildsIDCursor)

	var buildIDs []int
	c, found := versions.Cache.Get(cacheKey)
	if found {
		buildIDs = c.([]int)
	} else {
		builder := psql.Select("build_id").
			From("successful_build_versions").
			Where(sq.Eq{
				"job_id":      jobID,
				"version_md5": version,
				"resource_id": resourceID,
			})

		if buildsIDCursor != 0 {
			builder = builder.Where(sq.Lt{"build_id": buildsIDCursor})
		}

		rows, err := builder.
			OrderBy("build_id DESC").
			Limit(algorithmLimitRows).
			RunWith(versions.Conn).
			Query()
		if err != nil {
			return nil, 0, err
		}

		for rows.Next() {
			var id int
			err := rows.Scan(&id)
			if err != nil {
				return nil, 0, err
			}

			buildIDs = append(buildIDs, id)
		}

		versions.Cache.Set(cacheKey, buildIDs, gocache.DefaultExpiration)
	}

	var newBuildIDCursor int
	if len(buildIDs) > 0 {
		newBuildIDCursor = buildIDs[len(buildIDs)-1]
	}

	return buildIDs, newBuildIDCursor, nil
}

func (versions VersionsDB) BuildOutputs(buildID int) ([]AlgorithmOutput, error) {
	cacheKey := fmt.Sprintf("bo%d", buildID)

	c, found := versions.Cache.Get(cacheKey)
	if found {
		return c.([]AlgorithmOutput), nil
	}

	uniqOutputs := map[string]AlgorithmOutput{}
	rows, err := psql.Select("name", "resource_id", "version_md5").
		From("build_resource_config_version_inputs").
		Where(sq.Eq{"build_id": buildID}).
		RunWith(versions.Conn).
		Query()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var output AlgorithmOutput
		err := rows.Scan(&output.InputName, &output.ResourceID, &output.Version)
		if err != nil {
			return nil, err
		}

		uniqOutputs[output.InputName] = output
	}

	rows, err = psql.Select("name", "resource_id", "version_md5").
		From("build_resource_config_version_outputs").
		Where(sq.Eq{"build_id": buildID}).
		RunWith(versions.Conn).
		Query()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var output AlgorithmOutput
		err := rows.Scan(&output.InputName, &output.ResourceID, &output.Version)
		if err != nil {
			return nil, err
		}

		uniqOutputs[output.InputName] = output
	}

	outputs := []AlgorithmOutput{}
	for _, o := range uniqOutputs {
		outputs = append(outputs, o)
	}

	sort.Slice(outputs, func(i, j int) bool {
		return outputs[i].InputName > outputs[j].InputName
	})

	versions.Cache.Set(cacheKey, outputs, time.Hour)

	return outputs, nil
}

func (versions VersionsDB) SuccessfulBuildOutputs(buildID int) ([]AlgorithmOutput, error) {
	cacheKey := fmt.Sprintf("sbo%d", buildID)

	c, found := versions.Cache.Get(cacheKey)
	if found {
		return c.([]AlgorithmOutput), nil
	}

	uniqOutputs := map[string]AlgorithmOutput{}
	// TODO: prefer outputs over inputs for the same name
	rows, err := psql.Select("name", "resource_id", "version_md5").
		From("successful_build_versions").
		Where(sq.Eq{"build_id": buildID}).
		RunWith(versions.Conn).
		Query()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var output AlgorithmOutput
		err := rows.Scan(&output.InputName, &output.ResourceID, &output.Version)
		if err != nil {
			return nil, err
		}

		uniqOutputs[output.InputName] = output
	}

	outputs := []AlgorithmOutput{}
	for _, o := range uniqOutputs {
		outputs = append(outputs, o)
	}

	sort.Slice(outputs, func(i, j int) bool {
		return outputs[i].InputName > outputs[j].InputName
	})

	versions.Cache.Set(cacheKey, outputs, time.Hour)

	return outputs, nil
}

func (versions VersionsDB) FindVersionOfResource(resourceID int, v atc.Version) (ResourceVersion, bool, error) {
	versionJSON, err := json.Marshal(v)
	if err != nil {
		return "", false, nil
	}

	var version ResourceVersion
	err = psql.Select("rcv.version_md5").
		From("resource_config_versions rcv").
		Join("resources r ON r.resource_config_scope_id = rcv.resource_config_scope_id").
		Where(sq.Eq{
			"r.id": resourceID,
		}).
		Where(sq.Expr("rcv.version_md5 = md5(?)", versionJSON)).
		RunWith(versions.Conn).
		QueryRow().
		Scan(&version)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}

	return version, true, err
}

func (versions VersionsDB) LatestBuildID(jobID int) (int, bool, error) {
	cacheKey := fmt.Sprintf("lb%d", jobID)

	c, found := versions.Cache.Get(cacheKey)
	if found {
		return c.(int), c.(int) != 0, nil
	}

	var buildID int
	err := psql.Select("b.id").
		From("builds b").
		Where(sq.Eq{
			"b.job_id":    jobID,
			"b.scheduled": true,
		}).
		OrderBy("b.id DESC").
		Limit(100).
		RunWith(versions.Conn).
		QueryRow().
		Scan(&buildID)
	if err != nil {
		if err == sql.ErrNoRows {
			versions.Cache.Set(cacheKey, 0, gocache.DefaultExpiration)
			return 0, false, nil
		}
		return 0, false, err
	}

	versions.Cache.Set(cacheKey, buildID, gocache.DefaultExpiration)

	return buildID, true, nil
}

func (versions VersionsDB) NextEveryVersion(buildID int, resourceID int) (ResourceVersion, bool, error) {
	cacheKey := fmt.Sprintf("nev%d-%d", buildID, resourceID)

	c, found := versions.Cache.Get(cacheKey)
	if found {
		return c.(ResourceVersion), c.(ResourceVersion) != "", nil
	}

	tx, err := versions.Conn.Begin()
	if err != nil {
		return "", false, err
	}

	defer tx.Rollback()

	var checkOrder int
	err = psql.Select("rcv.check_order").
		From("build_resource_config_version_inputs i").
		Join("resource_config_versions rcv ON rcv.resource_config_scope_id = (SELECT resource_config_scope_id FROM resources WHERE id = ?)", resourceID).
		Where(sq.Expr("i.version_md5 = rcv.version_md5")).
		Where(sq.Eq{"i.build_id": buildID}).
		RunWith(tx).
		QueryRow().
		Scan(&checkOrder)
	if err != nil {
		if err == sql.ErrNoRows {
			version, found, err := versions.latestVersionOfResource(tx, resourceID)
			if err != nil {
				return "", false, err
			}

			if !found {
				versions.Cache.Set(cacheKey, "", gocache.DefaultExpiration)
				return "", false, nil
			}

			err = tx.Commit()
			if err != nil {
				return "", false, err
			}

			versions.Cache.Set(cacheKey, version, gocache.DefaultExpiration)

			return version, true, nil
		}

		return "", false, err
	}

	var nextVersion ResourceVersion
	err = psql.Select("rcv.version_md5").
		From("resource_config_versions rcv").
		Where(sq.Expr("rcv.resource_config_scope_id = (SELECT resource_config_scope_id FROM resources WHERE id = ?)", resourceID)).
		Where(sq.Expr("NOT EXISTS (SELECT 1 FROM resource_disabled_versions WHERE resource_id = ? AND version_md5 = rcv.version_md5)", resourceID)).
		Where(sq.Gt{"rcv.check_order": checkOrder}).
		OrderBy("rcv.check_order ASC").
		Limit(1).
		RunWith(tx).
		QueryRow().
		Scan(&nextVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			err = psql.Select("rcv.version_md5").
				From("resource_config_versions rcv").
				Where(sq.Expr("rcv.resource_config_scope_id = (SELECT resource_config_scope_id FROM resources WHERE id = ?)", resourceID)).
				Where(sq.Expr("NOT EXISTS (SELECT 1 FROM resource_disabled_versions WHERE resource_id = ? AND version_md5 = rcv.version_md5)", resourceID)).
				Where(sq.LtOrEq{"rcv.check_order": checkOrder}).
				OrderBy("rcv.check_order DESC").
				Limit(1).
				RunWith(tx).
				QueryRow().
				Scan(&nextVersion)
			if err != nil {
				if err == sql.ErrNoRows {
					versions.Cache.Set(cacheKey, "", gocache.DefaultExpiration)
					return "", false, nil
				}
				return "", false, err
			}
		} else {
			return "", false, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return "", false, err
	}

	versions.Cache.Set(cacheKey, nextVersion, gocache.DefaultExpiration)

	return nextVersion, true, nil
}

func (versions VersionsDB) LatestConstraintBuildID(buildID int, passedJobID int) (int, bool, error) {
	cacheKey := fmt.Sprintf("lcb%d-%d", buildID, passedJobID)

	c, found := versions.Cache.Get(cacheKey)
	if found {
		return c.(int), c.(int) != 0, nil
	}

	var latestBuildID int

	err := psql.Select("p.from_build_id").
		From("build_pipes p").
		Join("builds b ON b.id = p.from_build_id").
		Where(sq.Eq{
			"p.to_build_id": buildID,
			"b.job_id":      passedJobID,
		}).
		RunWith(versions.Conn).
		QueryRow().
		Scan(&latestBuildID)
	if err != nil {
		if err == sql.ErrNoRows {
			versions.Cache.Set(cacheKey, 0, gocache.DefaultExpiration)
			return 0, false, nil
		}

		return 0, false, err
	}

	versions.Cache.Set(cacheKey, latestBuildID, gocache.DefaultExpiration)
	return latestBuildID, true, nil
}

type paginatedBuilds struct {
	builder sq.SelectBuilder
	column  string

	buildIDs []int
	offset   int
}

func (bs paginatedBuilds) Next() (int, bool, error) {
	if bs.offset+1 > len(bs.buildIDs) {
		// we've reached the end, or we haven't fetched yet

		builder := bs.builder

		if len(bs.buildIDs) > 0 {
			builder = bs.builder.Where(sq.Lt{
				bs.column: bs.buildIDs[len(bs.buildIDs)-1],
			})
		}

		bs.buildIDs = []int{}
		bs.offset = 0

		rows, err := builder.Query()
		if err != nil {
			return 0, false, err
		}

		for rows.Next() {
			var buildID int

			err = rows.Scan(&buildID)
			if err != nil {
				return 0, false, err
			}

			bs.buildIDs = append(bs.buildIDs, buildID)
		}
	}

	if len(bs.buildIDs) == 0 {
		// there are no more builds
		return 0, false, nil
	}

	id := bs.buildIDs[bs.offset]
	bs.offset++

	return id, true, nil
}

func (versions VersionsDB) UnusedBuilds(buildID int, jobID int, buildsIDCursor int) ([]int, int, error) {
	cacheKey := fmt.Sprintf("ub%d-%d-%d", buildID, jobID, buildsIDCursor)
	var newBuildIDCursor int
	var buildIDs []int
	c, found := versions.Cache.Get(cacheKey)
	if found {
		buildIDs = c.([]int)
	} else {
		if buildsIDCursor == 0 {
			rows, err := psql.Select("id").
				From("builds").
				Where(sq.And{
					sq.Gt{"id": buildID},
					sq.Eq{
						"job_id": jobID,
						"status": "succeeded",
					},
				}).
				OrderBy("id ASC").
				RunWith(versions.Conn).
				Query()
			if err != nil {
				return nil, 0, err
			}

			for rows.Next() {
				var buildID int

				err = rows.Scan(&buildID)
				if err != nil {
					return nil, 0, err
				}

				buildIDs = append(buildIDs, buildID)
			}

		}

		builder := psql.Select("id").
			From("builds").
			Where(sq.And{
				sq.LtOrEq{"id": buildID},
				sq.Eq{
					"job_id": jobID,
					"status": "succeeded",
				},
			})

		if buildsIDCursor != 0 {
			builder = builder.Where(sq.Lt{"id": buildsIDCursor})
		}

		rows, err := builder.OrderBy("id DESC").
			Limit(algorithmLimitRows).
			RunWith(versions.Conn).
			Query()
		if err != nil {
			return nil, 0, err
		}

		for rows.Next() {
			var buildID int

			err = rows.Scan(&buildID)
			if err != nil {
				return nil, 0, err
			}

			buildIDs = append(buildIDs, buildID)
		}
	}

	versions.Cache.Set(cacheKey, buildIDs, gocache.DefaultExpiration)

	if len(buildIDs) > 0 {
		newBuildIDCursor = buildIDs[len(buildIDs)-1]
	}

	return buildIDs, newBuildIDCursor, nil
}

func (versions VersionsDB) UnusedBuildsVersionConstrained(buildID int, jobID int, version ResourceVersion, resourceID int, buildsIDCursor int) ([]int, int, error) {
	cacheKey := fmt.Sprintf("ubvc%d-%d-%s-%d", buildID, jobID, version, buildsIDCursor)

	var newBuildIDCursor int
	var buildIDs []int
	c, found := versions.Cache.Get(cacheKey)
	if found {
		buildIDs = c.([]int)
	} else {
		if buildsIDCursor == 0 {
			rows, err := psql.Select("build_id").
				From("successful_build_versions").
				Where(sq.Eq{
					"job_id":      jobID,
					"version_md5": version,
					"resource_id": resourceID,
				}).
				Where(sq.Gt{
					"build_id": buildID,
				}).
				OrderBy("build_id ASC").
				RunWith(versions.Conn).
				Query()
			if err != nil {
				return nil, 0, err
			}

			for rows.Next() {
				var buildID int

				err = rows.Scan(&buildID)
				if err != nil {
					return nil, 0, err
				}

				buildIDs = append(buildIDs, buildID)
			}
		}

		builder := psql.Select("build_id").
			From("successful_build_versions").
			Where(sq.Eq{
				"job_id":      jobID,
				"version_md5": version,
				"resource_id": resourceID,
			}).
			Where(sq.LtOrEq{
				"build_id": buildID,
			})

		if buildsIDCursor != 0 {
			builder = builder.Where(sq.Lt{"build_id": buildsIDCursor})
		}

		rows, err := builder.OrderBy("build_id DESC").
			RunWith(versions.Conn).
			Query()
		if err != nil {
			return nil, 0, err
		}

		for rows.Next() {
			var buildID int

			err = rows.Scan(&buildID)
			if err != nil {
				return nil, 0, err
			}

			buildIDs = append(buildIDs, buildID)
		}

		versions.Cache.Set(cacheKey, buildIDs, gocache.DefaultExpiration)
	}

	if len(buildIDs) > 0 {
		newBuildIDCursor = buildIDs[len(buildIDs)-1]
	}

	return buildIDs, newBuildIDCursor, nil
}

// Order passed jobs by whether or not the build pipes of the current job has a
// fromBuildID of the passed job. If there are multiple passed jobs that have a
// build pipe to the current job, then order them by the least number of
// builds. If there are jobs with the same number of builds, order
// alphabetically.

//TODO: turn this into a single query
func (versions VersionsDB) OrderPassedJobs(currentJobID int, jobs JobSet) ([]int, error) {
	var jobIDs []int
	for id, _ := range jobs {
		jobIDs = append(jobIDs, id)
	}

	sort.Ints(jobIDs)

	return jobIDs, nil

	// latestBuildID, found, err := versions.LatestBuildID(currentJobID)
	// if err != nil {
	// 	return nil, err
	// }

	// buildPipeJobs := make(map[int]bool)

	// if found {
	// 	cacheKey := fmt.Sprintf("bpj%d", latestBuildID)

	// 	c, found := versions.Cache.Get(cacheKey)
	// 	if found {
	// 		buildPipeJobs = c.(map[int]bool)
	// 	} else {
	// 		rows, err := psql.Select("b.job_id").
	// 			From("builds b").
	// 			Join("build_pipes bp ON bp.from_build_id = b.id").
	// 			Where(sq.Eq{"bp.to_build_id": latestBuildID}).
	// 			RunWith(versions.Conn).
	// 			Query()
	// 		if err != nil {
	// 			return nil, err
	// 		}

	// 		for rows.Next() {
	// 			var jobID int

	// 			err = rows.Scan(&jobID)
	// 			if err != nil {
	// 				return nil, err
	// 			}

	// 			buildPipeJobs[jobID] = true
	// 		}

	// 		versions.Cache.Set(cacheKey, buildPipeJobs, gocache.DefaultExpiration)
	// 	}
	// }

	// jobToBuilds := map[int]int{}
	// for job, _ := range jobs {
	// 	cacheKey := fmt.Sprintf("bj%d", job)

	// 	c, found := versions.Cache.Get(cacheKey)
	// 	if found {
	// 		jobToBuilds[job] = c.(int)
	// 	} else {
	// 		var buildNum int
	// 		err := psql.Select("COUNT(id)").
	// 			From("builds").
	// 			Where(sq.Eq{"job_id": job}).
	// 			RunWith(versions.Conn).
	// 			QueryRow().
	// 			Scan(&buildNum)
	// 		if err != nil {
	// 			return nil, err
	// 		}

	// 		jobToBuilds[job] = buildNum

	// 		versions.Cache.Set(cacheKey, buildNum, gocache.DefaultExpiration)
	// 	}
	// }

	// type jobBuilds struct {
	// 	jobID    int
	// 	buildNum int
	// }

	// var orderedJobBuilds []jobBuilds
	// for j, b := range jobToBuilds {
	// 	orderedJobBuilds = append(orderedJobBuilds, jobBuilds{j, b})
	// }

	// sort.Slice(orderedJobBuilds, func(i, j int) bool {
	// 	if buildPipeJobs[orderedJobBuilds[i].jobID] == buildPipeJobs[orderedJobBuilds[j].jobID] {
	// 		if orderedJobBuilds[i].buildNum == orderedJobBuilds[j].buildNum {
	// 			return orderedJobBuilds[i].jobID < orderedJobBuilds[j].jobID
	// 		}
	// 		return orderedJobBuilds[i].buildNum < orderedJobBuilds[j].buildNum
	// 	}

	// 	return buildPipeJobs[orderedJobBuilds[i].jobID]
	// })

	// orderedJobs := []int{}
	// for _, jobBuild := range orderedJobBuilds {
	// 	orderedJobs = append(orderedJobs, jobBuild.jobID)
	// }

	// return orderedJobs, nil
}

func (versions VersionsDB) latestVersionOfResource(tx Tx, resourceID int) (ResourceVersion, bool, error) {
	cacheKey := fmt.Sprintf("lv%d", resourceID)

	c, found := versions.Cache.Get(cacheKey)
	if found {
		return c.(ResourceVersion), c.(ResourceVersion) != "", nil
	}

	var scopeID int
	err := psql.Select("resource_config_scope_id").
		From("resources").
		Where(sq.Eq{"id": resourceID}).
		RunWith(tx).
		QueryRow().
		Scan(&scopeID)
	if err != nil {
		if err == sql.ErrNoRows {
			versions.Cache.Set(cacheKey, 0, gocache.DefaultExpiration)
			return "", false, nil
		}
		return "", false, err
	}

	var version ResourceVersion
	err = psql.Select("version_md5").
		From("resource_config_versions").
		Where(sq.Eq{"resource_config_scope_id": scopeID}).
		Where(sq.Expr("version_md5 NOT IN (SELECT version_md5 FROM resource_disabled_versions WHERE resource_id = ?)", resourceID)).
		OrderBy("check_order DESC").
		Limit(1).
		RunWith(tx).
		QueryRow().
		Scan(&version)
	if err != nil {
		if err == sql.ErrNoRows {
			versions.Cache.Set(cacheKey, version, gocache.DefaultExpiration)
			return "", false, nil
		}
		return "", false, err
	}

	versions.Cache.Set(cacheKey, version, gocache.DefaultExpiration)

	return version, true, nil
}
