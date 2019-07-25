package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	gocache "github.com/patrickmn/go-cache"
)

type JobNotFoundError struct {
	ID int
}

func (e JobNotFoundError) Error() string {
	return fmt.Sprintf("job ID %d is not found", e.ID)
}

type VersionsDB struct {
	Conn      Conn
	LimitRows int

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

func (versions VersionsDB) SuccessfulBuilds(jobID int) PaginatedBuilds {
	builder := psql.Select("b.id").
		From("builds b").
		Where(sq.Eq{
			"b.job_id": jobID,
			"b.status": "succeeded",
		}).
		OrderBy("b.id DESC")

	return PaginatedBuilds{
		builder: builder,
		column:  "b.id",

		limitRows: versions.LimitRows,
		conn:      versions.Conn,
	}
}

func (versions VersionsDB) SuccessfulBuildsVersionConstrained(jobID int, constrainingCandidates map[string][]string) (PaginatedBuilds, error) {
	versionsJSON, err := json.Marshal(constrainingCandidates)
	if err != nil {
		return PaginatedBuilds{}, err
	}

	builder := psql.Select("build_id").
		From("successful_build_outputs").
		Where(sq.Expr("outputs @> ?::jsonb", versionsJSON)).
		Where(sq.Eq{
			"job_id": jobID,
		}).
		OrderBy("build_id DESC")

	return PaginatedBuilds{
		builder: builder,
		column:  "build_id",

		limitRows: versions.LimitRows,
		conn:      versions.Conn,
	}, nil
}

func (versions VersionsDB) BuildOutputs(buildID int) ([]AlgorithmOutput, error) {
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

	return outputs, nil
}

func (versions VersionsDB) SuccessfulBuildOutputs(buildID int) ([]AlgorithmVersion, error) {
	cacheKey := fmt.Sprintf("o%d", buildID)

	c, found := versions.Cache.Get(cacheKey)
	if found {
		return c.([]AlgorithmVersion), nil
	}

	// TODO: prefer outputs over inputs for the same name
	var outputsJSON string
	err := psql.Select("outputs").
		From("successful_build_outputs").
		Where(sq.Eq{"build_id": buildID}).
		RunWith(versions.Conn).
		QueryRow().
		Scan(&outputsJSON)
	if err != nil {
		return nil, err
	}

	outputs := map[string][]string{}
	err = json.Unmarshal([]byte(outputsJSON), &outputs)
	if err != nil {
		return nil, err
	}

	algorithmOutputs := []AlgorithmVersion{}
	for resID, versions := range outputs {
		for _, version := range versions {
			resourceID, err := strconv.Atoi(resID)
			if err != nil {
				return nil, err
			}

			algorithmOutputs = append(algorithmOutputs, AlgorithmVersion{
				ResourceID: resourceID,
				Version:    ResourceVersion(version),
			})
		}
	}

	sort.Slice(algorithmOutputs, func(i, j int) bool {
		return algorithmOutputs[i].ResourceID < algorithmOutputs[j].ResourceID
	})

	versions.Cache.Set(cacheKey, algorithmOutputs, time.Hour)

	return algorithmOutputs, nil
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
			return 0, false, nil
		}
		return 0, false, err
	}

	return buildID, true, nil
}

func (versions VersionsDB) NextEveryVersion(buildID int, resourceID int) (ResourceVersion, bool, error) {
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
				return "", false, nil
			}

			err = tx.Commit()
			if err != nil {
				return "", false, err
			}

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

	return nextVersion, true, nil
}

func (versions VersionsDB) LatestBuildPipes(buildID int, passedJobs map[int]bool) (map[int]int, error) {
	rows, err := psql.Select("p.from_build_id", "b.job_id").
		From("build_pipes p").
		Join("builds b ON b.id = p.from_build_id").
		Where(sq.Eq{
			"p.to_build_id": buildID,
		}).
		RunWith(versions.Conn).
		Query()
	if err != nil {
		return nil, err
	}

	jobToBuildPipes := map[int]int{}
	for rows.Next() {
		var buildID int
		var jobID int

		err = rows.Scan(&buildID, &jobID)
		if err != nil {
			return nil, err
		}

		if passedJobs[jobID] {
			jobToBuildPipes[jobID] = buildID
		}
	}

	return jobToBuildPipes, nil
}

func (versions VersionsDB) UnusedBuilds(buildID int, jobID int) (PaginatedBuilds, error) {
	var buildIDs []int
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
		Limit(algorithmLimitRows).
		RunWith(versions.Conn).
		Query()
	if err != nil {
		return PaginatedBuilds{}, err
	}

	for rows.Next() {
		var buildID int

		err = rows.Scan(&buildID)
		if err != nil {
			return PaginatedBuilds{}, err
		}

		buildIDs = append(buildIDs, buildID)
	}

	builder := psql.Select("id").
		From("builds").
		Where(sq.And{
			sq.LtOrEq{"id": buildID},
			sq.Eq{
				"job_id": jobID,
				"status": "succeeded",
			},
		}).
		OrderBy("id DESC")

	return PaginatedBuilds{
		builder:  builder,
		buildIDs: buildIDs,
		column:   "id",

		limitRows: versions.LimitRows,
		conn:      versions.Conn,
	}, nil
}

func (versions VersionsDB) UnusedBuildsVersionConstrained(buildID int, jobID int, constrainingCandidates map[string][]string) (PaginatedBuilds, error) {
	var buildIDs []int
	versionsJSON, err := json.Marshal(constrainingCandidates)
	if err != nil {
		return PaginatedBuilds{}, err
	}

	rows, err := psql.Select("build_id").
		From("successful_build_outputs").
		Where(sq.Expr("outputs @> ?::jsonb", versionsJSON)).
		Where(sq.Eq{
			"job_id": jobID,
		}).
		Where(sq.Gt{
			"build_id": buildID,
		}).
		OrderBy("build_id ASC").
		RunWith(versions.Conn).
		Query()
	if err != nil {
		return PaginatedBuilds{}, err
	}

	for rows.Next() {
		var buildID int

		err = rows.Scan(&buildID)
		if err != nil {
			return PaginatedBuilds{}, err
		}

		buildIDs = append(buildIDs, buildID)
	}

	builder := psql.Select("build_id").
		From("successful_build_outputs").
		Where(sq.Expr("outputs @> ?::jsonb", versionsJSON)).
		Where(sq.Eq{
			"job_id": jobID,
		}).
		Where(sq.LtOrEq{
			"build_id": buildID,
		}).
		OrderBy("build_id DESC")

	return PaginatedBuilds{
		builder:  builder,
		buildIDs: buildIDs,
		column:   "build_id",

		limitRows: versions.LimitRows,
		conn:      versions.Conn,
	}, nil

}

func (versions VersionsDB) OrderPassedJobs(currentJobID int, jobs JobSet) ([]int, error) {
	var jobIDs []int
	for id, _ := range jobs {
		jobIDs = append(jobIDs, id)
	}

	sort.Ints(jobIDs)

	return jobIDs, nil
}

func (versions VersionsDB) latestVersionOfResource(tx Tx, resourceID int) (ResourceVersion, bool, error) {
	var scopeID int
	err := psql.Select("resource_config_scope_id").
		From("resources").
		Where(sq.Eq{"id": resourceID}).
		RunWith(tx).
		QueryRow().
		Scan(&scopeID)
	if err != nil {
		if err == sql.ErrNoRows {
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
			return "", false, nil
		}
		return "", false, err
	}

	return version, true, nil
}

type PaginatedBuilds struct {
	builder sq.SelectBuilder
	column  string

	buildIDs []int
	offset   int

	limitRows int
	conn      Conn
}

func (bs *PaginatedBuilds) Next() (int, bool, error) {
	if bs.offset+1 > len(bs.buildIDs) {
		builder := bs.builder

		if len(bs.buildIDs) > 0 {
			builder = bs.builder.Where(sq.Lt{
				bs.column: bs.buildIDs[len(bs.buildIDs)-1],
			})
		}

		bs.buildIDs = []int{}
		bs.offset = 0

		rows, err := builder.
			Limit(uint64(bs.limitRows)).
			RunWith(bs.conn).
			Query()
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

		if len(bs.buildIDs) == 0 {
			return 0, false, nil
		}
	}

	id := bs.buildIDs[bs.offset]
	bs.offset++

	return id, true, nil
}
