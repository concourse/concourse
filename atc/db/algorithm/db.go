package algorithm

import (
	"database/sql"
	"log"
	"strconv"

	sq "github.com/Masterminds/squirrel"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type VersionsDB struct {
	Runner sq.Runner

	JobIDs      map[string]int
	ResourceIDs map[string]int
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

	return exists, nil
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

func (db VersionsDB) AllVersionsOfResource(resourceID int) (VersionCandidates, error) {
	var scopeID int
	err := psql.Select("resource_config_scope_id").
		From("resources").
		Where(sq.Eq{"id": resourceID}).
		RunWith(db.Runner).
		QueryRow().
		Scan(&scopeID)
	if err != nil {
		return VersionCandidates{}, err
	}

	return VersionCandidates{
		runner: db.Runner,
		versionsQuery: psql.Select("v.id", "v.check_order").
			From("resource_config_versions v").
			Where(sq.Eq{"v.resource_config_scope_id": scopeID}).
			RunWith(db.Runner),
	}, nil
}

func (db VersionsDB) FindVersionOfResource(resourceID int, versionID int) (VersionCandidates, error) {
	var scopeID int
	err := psql.Select("resource_config_scope_id").
		From("resources").
		Where(sq.Eq{"id": resourceID}).
		RunWith(db.Runner).
		QueryRow().
		Scan(&scopeID)
	if err != nil {
		return VersionCandidates{}, err
	}

	return VersionCandidates{
		runner: db.Runner,
		versionsQuery: psql.Select("v.id", "v.check_order").
			From("resource_config_versions v").
			Where(sq.Eq{
				"v.resource_config_scope_id": scopeID,
				"v.id":                       versionID,
			}).
			RunWith(db.Runner),
	}, nil
}

func (db VersionsDB) VersionsOfResourcePassedJobs(resourceID int, passed JobSet) (VersionCandidates, error) {
	var scopeID int
	err := psql.Select("resource_config_scope_id").
		From("resources").
		Where(sq.Eq{"id": resourceID}).
		RunWith(db.Runner).
		QueryRow().
		Scan(&scopeID)
	if err != nil {
		return VersionCandidates{}, err
	}

	var jobIDs []int
	for jobID := range passed {
		jobIDs = append(jobIDs, jobID)
	}

	// TODO: look at inputs of succeeded builds, too
	query := psql.Select("v.id", "v.check_order").
		Distinct(). // TODO: verify that this is necessary
		From("resource_config_versions v").
		Where(sq.Eq{"v.resource_config_scope_id": scopeID}).
		RunWith(db.Runner)

	for _, id := range jobIDs {
		joinID := strconv.Itoa(id)
		o := "o" + joinID
		b := "b" + joinID

		query = query.
			Join("build_resource_config_version_outputs "+o+" ON "+o+".resource_id = ? AND "+o+".version_md5 = v.version_md5", resourceID).
			Join("builds "+b+" ON "+b+".job_id = ? AND "+b+".id = "+o+".build_id", id)
	}

	builds := map[int]BuildSet{}
	for _, id := range jobIDs {
		builds[id] = BuildSet{}

		q := psql.Select("b.id").
			From("builds b").
			// Join("build_resource_config_version_outputs o ON o.build_id = b.id AND b.resource_id = ?", resourceID).
			Where(sq.Eq{"b.job_id": id}).
			Where("EXISTS (SELECT 1 FROM build_resource_config_version_outputs WHERE build_id = b.id AND resource_id = ?)", resourceID)

		log.Println(q.ToSql())
		rows, err := q.
			RunWith(db.Runner).
			Query()
		if err != nil {
			return VersionCandidates{}, err
		}

		for rows.Next() {
			var buildID int
			err = rows.Scan(&buildID)
			if err != nil {
				return VersionCandidates{}, err
			}

			log.Println("thank you, next", buildID)

			builds[id][buildID] = struct{}{}
		}
	}

	// TODO: look at inputs of succeeded builds, too
	return VersionCandidates{
		runner:        db.Runner,
		versionsQuery: query.RunWith(db.Runner),
		buildIDs:      builds,
	}, nil
}

func (db VersionsDB) GetLatestBuildID(jobID int) (int, error) {
	var buildID int
	err := psql.Select("b.id").
		From("builds b").
		Where(sq.Eq{
			"b.job_id": jobID,
		}).
		OrderBy("b.id DESC").
		Limit(1).
		RunWith(db.Runner).
		QueryRow().
		Scan(&buildID)
	if err != nil {
		return 0, err
	}

	return buildID, nil
}

func (db VersionsDB) NextEveryVersion(buildID int, resourceID int) (int, error) {
	// Grab the version_md5 for the build input using the buildID and resourceID
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
		return 0, err
	}

	// Query for all the versions of the resource with a high check_order of the version_md5 ASC
	// Query for if the next version is disabled?
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

	// Query for all versions of resource with check_order =< the version_md5 DESC
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
