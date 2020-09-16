package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/tracing"
	gocache "github.com/patrickmn/go-cache"
	"go.opentelemetry.io/otel/api/key"
	"go.opentelemetry.io/otel/api/trace"
)

type VersionsDB struct {
	conn      Conn
	limitRows int

	cache *gocache.Cache
}

func NewVersionsDB(conn Conn, limitRows int, cache *gocache.Cache) VersionsDB {
	return VersionsDB{
		conn:      conn,
		limitRows: limitRows,
		cache:     cache,
	}
}

func (versions VersionsDB) IsFirstOccurrence(ctx context.Context, jobID int, inputName string, versionMD5 ResourceVersion, resourceId int) (bool, error) {
	var exists bool
	err := versions.conn.QueryRowContext(ctx, `
		WITH builds_of_job AS (
			SELECT id FROM builds WHERE job_id = $1
		)
		SELECT EXISTS (
			SELECT 1
			FROM build_resource_config_version_inputs i
			JOIN builds_of_job b ON b.id = i.build_id
			WHERE i.name = $2
			AND i.version_md5 = $3
			AND i.resource_id = $4
		)`, jobID, inputName, versionMD5, resourceId).
		Scan(&exists)
	if err != nil {
		return false, err
	}

	return !exists, nil
}

func (versions VersionsDB) VersionIsDisabled(ctx context.Context, resourceID int, versionMD5 ResourceVersion) (bool, error) {
	var exists bool
	err := versions.conn.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM resource_disabled_versions
			WHERE resource_id = $1
			AND version_md5 = $2
		)`, resourceID, versionMD5).
		Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (versions VersionsDB) LatestVersionOfResource(ctx context.Context, resourceID int) (ResourceVersion, bool, error) {
	tx, err := versions.conn.Begin()
	if err != nil {
		return "", false, err
	}

	defer tx.Rollback()

	version, found, err := versions.latestVersionOfResource(ctx, tx, resourceID)
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

func (versions VersionsDB) SuccessfulBuilds(ctx context.Context, jobID int) PaginatedBuilds {
	builder := psql.Select("id", "rerun_of").
		From("builds").
		Where(sq.Eq{
			"job_id": jobID,
			"status": "succeeded",
		}).
		OrderBy("COALESCE(rerun_of, id) DESC, id DESC")

	return PaginatedBuilds{
		builder: builder,
		column:  "id",
		jobID:   jobID,

		limitRows: versions.limitRows,
		conn:      versions.conn,
	}
}

func (versions VersionsDB) SuccessfulBuildsVersionConstrained(
	ctx context.Context,
	jobID int,
	constrainingCandidates map[string][]string,
) (PaginatedBuilds, error) {
	versionsJSON, err := json.Marshal(constrainingCandidates)
	if err != nil {
		return PaginatedBuilds{}, err
	}

	builder := psql.Select("build_id", "rerun_of").
		From("successful_build_outputs").
		Where(sq.Expr("outputs @> ?::jsonb", versionsJSON)).
		Where(sq.Eq{
			"job_id": jobID,
		}).
		OrderBy("COALESCE(rerun_of, build_id) DESC, build_id DESC")

	return PaginatedBuilds{
		builder: builder,
		column:  "build_id",
		jobID:   jobID,

		limitRows: versions.limitRows,
		conn:      versions.conn,
	}, nil
}

type resourceOutputs struct {
	ResourceID int
	Versions   []string
}

func (versions VersionsDB) SuccessfulBuildOutputs(ctx context.Context, buildID int) ([]AlgorithmVersion, error) {
	cacheKey := fmt.Sprintf("o%d", buildID)

	c, found := versions.cache.Get(cacheKey)
	if found {
		return c.([]AlgorithmVersion), nil
	}

	var outputsJSON string
	err := psql.Select("outputs").
		From("successful_build_outputs").
		Where(sq.Eq{"build_id": buildID}).
		RunWith(versions.conn).
		QueryRowContext(ctx).
		Scan(&outputsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			outputsJSON, err = versions.migrateSingle(ctx, buildID)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	outputs := map[string][]string{}
	err = json.Unmarshal([]byte(outputsJSON), &outputs)
	if err != nil {
		return nil, err
	}

	byResourceID := []resourceOutputs{}
	for resourceIDStr, versions := range outputs {
		resourceID, err := strconv.Atoi(resourceIDStr)
		if err != nil {
			return nil, err
		}

		byResourceID = append(byResourceID, resourceOutputs{
			ResourceID: resourceID,
			Versions:   versions,
		})
	}

	sort.Slice(byResourceID, func(i, j int) bool {
		return byResourceID[i].ResourceID < byResourceID[j].ResourceID
	})

	algorithmOutputs := []AlgorithmVersion{}
	for _, outputs := range byResourceID {
		for _, version := range outputs.Versions {
			algorithmOutputs = append(algorithmOutputs, AlgorithmVersion{
				ResourceID: outputs.ResourceID,
				Version:    ResourceVersion(version),
			})
		}
	}

	versions.cache.Set(cacheKey, algorithmOutputs, time.Hour)

	return algorithmOutputs, nil
}

func (versions VersionsDB) VersionExists(ctx context.Context, resourceID int, versionMD5 ResourceVersion) (bool, error) {
	var exists bool
	err := versions.conn.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM resource_config_versions v
			JOIN resources r ON r.resource_config_scope_id = v.resource_config_scope_id
			WHERE r.id = $1
			AND v.version_md5 = $2
		)`, resourceID, versionMD5).
		Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (versions VersionsDB) FindVersionOfResource(ctx context.Context, resourceID int, v atc.Version) (ResourceVersion, bool, error) {
	versionJSON, err := json.Marshal(v)
	if err != nil {
		return "", false, nil
	}

	cacheKey := fmt.Sprintf("v%d-%s", resourceID, versionJSON)

	c, found := versions.cache.Get(cacheKey)
	if found {
		return c.(ResourceVersion), true, nil
	}

	var version ResourceVersion
	err = psql.Select("rcv.version_md5").
		From("resource_config_versions rcv").
		Join("resources r ON r.resource_config_scope_id = rcv.resource_config_scope_id").
		Where(sq.Eq{
			"r.id": resourceID,
		}).
		Where(sq.Expr("rcv.version @> ?", versionJSON)).
		RunWith(versions.conn).
		QueryRowContext(ctx).
		Scan(&version)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}

	versions.cache.Set(cacheKey, version, time.Hour)

	return version, true, err
}

func (versions VersionsDB) NextEveryVersion(ctx context.Context, jobID int, resourceID int) (ResourceVersion, bool, bool, error) {
	tx, err := versions.conn.Begin()
	if err != nil {
		return "", false, false, err
	}

	defer tx.Rollback()

	var checkOrder int
	err = tx.QueryRowContext(ctx, `
		SELECT rcv.check_order
		FROM resource_config_versions rcv
		CROSS JOIN LATERAL (
			SELECT i.build_id
			FROM build_resource_config_version_inputs i
			CROSS JOIN LATERAL (
				SELECT b.id
				FROM builds b
				WHERE b.job_id = $1
				AND i.build_id = b.id
				LIMIT 1
			) AS build
			WHERE i.resource_id = $2
			AND i.version_md5 = rcv.version_md5
			LIMIT 1
		) AS inputs
		WHERE rcv.resource_config_scope_id = (SELECT resource_config_scope_id FROM resources WHERE id = $2)
		ORDER BY rcv.check_order DESC
		LIMIT 1;`, jobID, resourceID).Scan(&checkOrder)
	if err != nil {
		if err == sql.ErrNoRows {
			version, found, err := versions.latestVersionOfResource(ctx, tx, resourceID)
			if err != nil {
				return "", false, false, err
			}

			if !found {
				return "", false, false, nil
			}

			err = tx.Commit()
			if err != nil {
				return "", false, false, err
			}

			return version, false, true, nil
		}

		return "", false, false, err
	}

	var nextVersion ResourceVersion
	rows, err := psql.Select("rcv.version_md5").
		From("resource_config_versions rcv").
		Where(sq.Expr("rcv.resource_config_scope_id = (SELECT resource_config_scope_id FROM resources WHERE id = ?)", resourceID)).
		Where(sq.Expr("NOT EXISTS (SELECT 1 FROM resource_disabled_versions WHERE resource_id = ? AND version_md5 = rcv.version_md5)", resourceID)).
		Where(sq.Gt{"rcv.check_order": checkOrder}).
		OrderBy("rcv.check_order ASC").
		Limit(2).
		RunWith(tx).
		QueryContext(ctx)
	if err != nil {
		return "", false, false, err
	}

	if rows.Next() {
		err = rows.Scan(&nextVersion)
		if err != nil {
			return "", false, false, err
		}

		var hasNext bool
		if rows.Next() {
			hasNext = true
		}

		rows.Close()

		err = tx.Commit()
		if err != nil {
			return "", false, false, err
		}

		return nextVersion, hasNext, true, nil
	}

	err = psql.Select("rcv.version_md5").
		From("resource_config_versions rcv").
		Where(sq.Expr("rcv.resource_config_scope_id = (SELECT resource_config_scope_id FROM resources WHERE id = ?)", resourceID)).
		Where(sq.Expr("NOT EXISTS (SELECT 1 FROM resource_disabled_versions WHERE resource_id = ? AND version_md5 = rcv.version_md5)", resourceID)).
		Where(sq.LtOrEq{"rcv.check_order": checkOrder}).
		OrderBy("rcv.check_order DESC").
		Limit(1).
		RunWith(tx).
		QueryRowContext(ctx).
		Scan(&nextVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, false, nil
		}
		return "", false, false, err
	}

	err = tx.Commit()
	if err != nil {
		return "", false, false, err
	}

	return nextVersion, false, true, nil
}

func (versions VersionsDB) LatestBuildPipes(ctx context.Context, buildID int) (map[int]BuildCursor, error) {
	rows, err := psql.Select("p.from_build_id", "b.rerun_of", "b.job_id").
		From("build_pipes p").
		Join("builds b ON b.id = p.from_build_id").
		Where(sq.Eq{
			"p.to_build_id": buildID,
		}).
		RunWith(versions.conn).
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}

	jobToBuildPipes := map[int]BuildCursor{}
	for rows.Next() {
		var build BuildCursor
		var jobID int

		err = rows.Scan(&build.ID, &build.RerunOf, &jobID)
		if err != nil {
			return nil, err
		}

		jobToBuildPipes[jobID] = build
	}

	return jobToBuildPipes, nil
}

func (versions VersionsDB) LatestBuildUsingLatestVersion(ctx context.Context, jobID int, resourceID int) (int, bool, error) {
	var buildID int
	err := versions.conn.QueryRowContext(ctx, `
		SELECT inputs.build_id
		FROM resource_config_versions rcv
		CROSS JOIN LATERAL (
			SELECT i.build_id
			FROM build_resource_config_version_inputs i
			CROSS JOIN LATERAL (
				SELECT b.id
				FROM builds b
				WHERE b.job_id = $1
				AND i.build_id = b.id
				ORDER BY b.id DESC
				LIMIT 1
			) AS build
			WHERE i.resource_id = $2
			AND i.version_md5 = rcv.version_md5
			LIMIT 1
		) AS inputs
		WHERE rcv.resource_config_scope_id = (SELECT resource_config_scope_id FROM resources WHERE id = $2)
		ORDER BY rcv.check_order DESC
		LIMIT 1`, jobID, resourceID).Scan(&buildID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}

	return buildID, true, nil
}

func (versions VersionsDB) UnusedBuilds(ctx context.Context, jobID int, lastUsedBuild BuildCursor) (PaginatedBuilds, error) {
	builds, err := versions.newerBuilds(ctx, jobID, lastUsedBuild)
	if err != nil {
		return PaginatedBuilds{}, err
	}

	builder := psql.Select("id", "rerun_of").
		From("builds").
		Where(sq.And{
			sq.Eq{
				"job_id": jobID,
				"status": "succeeded",
			},
			sq.Or{
				sq.Eq{"id": lastUsedBuild.ID},
				lastUsedBuild.OlderBuilds("id"),
			},
		}).
		OrderBy("COALESCE(rerun_of, id) DESC, id DESC")

	return PaginatedBuilds{
		builder:      builder,
		builds:       builds,
		unusedBuilds: true,

		column: "id",
		jobID:  jobID,

		limitRows: versions.limitRows,
		conn:      versions.conn,
	}, nil
}

func (versions VersionsDB) UnusedBuildsVersionConstrained(ctx context.Context, jobID int, lastUsedBuild BuildCursor, constrainingCandidates map[string][]string) (PaginatedBuilds, error) {
	builds, err := versions.newerBuilds(ctx, jobID, lastUsedBuild)
	if err != nil {
		return PaginatedBuilds{}, err
	}

	versionsJSON, err := json.Marshal(constrainingCandidates)
	if err != nil {
		return PaginatedBuilds{}, err
	}

	builder := psql.Select("build_id", "rerun_of").
		From("successful_build_outputs").
		Where(sq.Expr("outputs @> ?::jsonb", versionsJSON)).
		Where(sq.Eq{
			"job_id": jobID,
		}).
		Where(sq.Or{
			sq.Eq{"build_id": lastUsedBuild.ID},
			lastUsedBuild.OlderBuilds("build_id"),
		}).
		OrderBy("COALESCE(rerun_of, build_id) DESC, build_id DESC")

	return PaginatedBuilds{
		builder:      builder,
		builds:       builds,
		unusedBuilds: true,

		column: "build_id",
		jobID:  jobID,

		limitRows: versions.limitRows,
		conn:      versions.conn,
	}, nil

}

func (versions VersionsDB) newerBuilds(ctx context.Context, jobID int, lastUsedBuild BuildCursor) ([]BuildCursor, error) {
	rows, err := psql.Select("id", "rerun_of").
		From("builds").
		Where(sq.And{
			sq.Eq{
				"job_id": jobID,
				"status": "succeeded",
			},
			lastUsedBuild.NewerBuilds("id"),
		}).
		OrderBy("COALESCE(rerun_of, id) ASC, id ASC").
		RunWith(versions.conn).
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}

	var builds []BuildCursor
	for rows.Next() {
		var build BuildCursor
		err = rows.Scan(&build.ID, &build.RerunOf)
		if err != nil {
			return nil, err
		}

		builds = append(builds, build)
	}

	return builds, nil
}

func (versions VersionsDB) latestVersionOfResource(ctx context.Context, tx Tx, resourceID int) (ResourceVersion, bool, error) {
	var scopeID sql.NullInt64
	err := psql.Select("resource_config_scope_id").
		From("resources").
		Where(sq.Eq{"id": resourceID}).
		RunWith(tx).
		QueryRowContext(ctx).
		Scan(&scopeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}

	if !scopeID.Valid {
		return "", false, nil
	}

	var version ResourceVersion
	err = psql.Select("version_md5").
		From("resource_config_versions").
		Where(sq.Eq{"resource_config_scope_id": scopeID}).
		Where(sq.Expr("version_md5 NOT IN (SELECT version_md5 FROM resource_disabled_versions WHERE resource_id = ?)", resourceID)).
		OrderBy("check_order DESC").
		Limit(1).
		RunWith(tx).
		QueryRowContext(ctx).
		Scan(&version)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}

	return version, true, nil
}

func (versions VersionsDB) migrateSingle(ctx context.Context, buildID int) (string, error) {
	ctx, span := tracing.StartSpan(ctx, "VersionsDB.migrateSingle", tracing.Attrs{})
	defer span.End()

	span.SetAttributes(key.New("buildID").Int(buildID))

	var outputs string
	err := versions.conn.QueryRowContext(ctx, `
		WITH builds_to_migrate AS (
			UPDATE builds
			SET needs_v6_migration = false
			WHERE id = $1
		)
			INSERT INTO successful_build_outputs (
				SELECT b.id, b.job_id, json_object_agg(sp.resource_id, sp.v), b.rerun_of
				FROM builds b
				JOIN (
					SELECT build_id, resource_id, json_agg(version_md5) AS v
					FROM (
						(
							SELECT build_id, resource_id, version_md5
							FROM build_resource_config_version_outputs o
							WHERE o.build_id = $1
						)
						UNION ALL
						(
							SELECT build_id, resource_id, version_md5
							FROM build_resource_config_version_inputs i
							WHERE i.build_id = $1
						)
				) AS agg GROUP BY build_id, resource_id) sp ON sp.build_id = b.id
				WHERE b.id = $1
				GROUP BY b.id, b.job_id, b.rerun_of
			)
			ON CONFLICT (build_id) DO UPDATE SET outputs = EXCLUDED.outputs
			RETURNING outputs
		`, buildID).
		Scan(&outputs)
	if err != nil {
		tracing.End(span, err)
		return "", err
	}

	span.AddEvent(ctx, "build migrated")

	return outputs, nil
}

type BuildCursor struct {
	ID      int
	RerunOf sql.NullInt64
}

func (cursor BuildCursor) OlderBuilds(idCol string) sq.Sqlizer {
	if cursor.RerunOf.Valid {
		return sq.Or{
			sq.Expr("COALESCE(rerun_of, "+idCol+") < ?", cursor.RerunOf.Int64),

			// include original build of the rerun
			sq.Eq{idCol: cursor.RerunOf.Int64},

			// include earlier reruns of the same build
			sq.And{
				sq.Eq{"rerun_of": cursor.RerunOf.Int64},
				sq.Lt{idCol: cursor.ID},
			},
		}
	} else {
		return sq.Expr("COALESCE(rerun_of, "+idCol+") < ?", cursor.ID)
	}
}

func (cursor BuildCursor) NewerBuilds(idCol string) sq.Sqlizer {
	if cursor.RerunOf.Valid {
		return sq.Or{
			sq.Expr("COALESCE(rerun_of, "+idCol+") > ?", cursor.RerunOf.Int64),
			sq.And{
				sq.Eq{"rerun_of": cursor.RerunOf.Int64},
				sq.Gt{idCol: cursor.ID},
			},
		}
	} else {
		return sq.Or{
			sq.Expr("COALESCE(rerun_of, "+idCol+") > ?", cursor.ID),

			// include reruns of the build
			sq.Eq{"rerun_of": cursor.ID},
		}
	}
}

type PaginatedBuilds struct {
	builder sq.SelectBuilder
	column  string

	unusedBuilds bool
	builds       []BuildCursor
	offset       int

	jobID int

	limitRows int
	conn      Conn
}

func (bs *PaginatedBuilds) Next(ctx context.Context) (int, bool, error) {
	if bs.offset+1 > len(bs.builds) {
		for {
			builder := bs.builder

			if len(bs.builds) > 0 {
				pageBoundary := bs.builds[len(bs.builds)-1]
				builder = builder.Where(pageBoundary.OlderBuilds(bs.column))
			}

			rows, err := builder.
				Limit(uint64(bs.limitRows)).
				RunWith(bs.conn).
				QueryContext(ctx)
			if err != nil {
				return 0, false, err
			}

			builds := []BuildCursor{}
			for rows.Next() {
				var build BuildCursor
				err = rows.Scan(&build.ID, &build.RerunOf)
				if err != nil {
					return 0, false, err
				}

				builds = append(builds, build)
			}

			if len(builds) == 0 {
				migrated, err := bs.migrateLimit(ctx)
				if err != nil {
					return 0, false, err
				}

				if !migrated {
					return 0, false, nil
				}
			} else {
				bs.builds = builds
				bs.offset = 0
				bs.unusedBuilds = false
				break
			}
		}
	}

	build := bs.builds[bs.offset]
	bs.offset++

	return build.ID, true, nil
}

func (bs *PaginatedBuilds) HasNext() bool {
	return bs.unusedBuilds && len(bs.builds)-bs.offset+1 > 0
}

func (bs *PaginatedBuilds) migrateLimit(ctx context.Context) (bool, error) {
	ctx, span := tracing.StartSpan(ctx, "PaginatedBuilds.migrateLimit", tracing.Attrs{})
	defer span.End()

	span.SetAttributes(key.New("jobID").Int(bs.jobID))

	buildsToMigrateQueryBuilder := psql.Select("id", "job_id", "rerun_of").
		From("builds").
		Where(sq.Eq{
			"job_id":             bs.jobID,
			"needs_v6_migration": true,
			"status":             "succeeded",
		}).
		OrderBy("COALESCE(rerun_of, id) DESC, id DESC").
		Limit(uint64(bs.limitRows))

	buildsToMigrateQuery, params, err := buildsToMigrateQueryBuilder.ToSql()
	if err != nil {
		tracing.End(span, err)
		return false, err
	}

	results, err := bs.conn.ExecContext(ctx, `
		WITH builds_to_migrate AS (`+buildsToMigrateQuery+`), migrated_outputs AS (
			INSERT INTO successful_build_outputs (
				SELECT bm.id, bm.job_id, json_object_agg(sp.resource_id, sp.v), bm.rerun_of
				FROM builds_to_migrate bm
				JOIN (
					SELECT build_id, resource_id, json_agg(version_md5) AS v
					FROM (
						(
							SELECT build_id, resource_id, version_md5
							FROM build_resource_config_version_outputs o
							JOIN builds_to_migrate bm ON bm.id = o.build_id
						)
						UNION ALL
						(
							SELECT build_id, resource_id, version_md5
							FROM build_resource_config_version_inputs i
							JOIN builds_to_migrate bm ON bm.id = i.build_id
						)
				) AS agg GROUP BY build_id, resource_id) sp ON sp.build_id = bm.id
				GROUP BY bm.id, bm.job_id, bm.rerun_of
			) ON CONFLICT (build_id) DO NOTHING
		)
		UPDATE builds
		SET needs_v6_migration = false
		WHERE id IN (SELECT id FROM builds_to_migrate)
	`, params...)
	if err != nil {
		tracing.End(span, err)
		return false, err
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		tracing.End(span, err)
		return false, err
	}

	trace.SpanFromContext(ctx).AddEvent(
		ctx,
		"builds migrated",
		key.New("rows").Int64(rowsAffected),
	)

	if rowsAffected == 0 {
		return false, nil
	}

	return true, nil
}
