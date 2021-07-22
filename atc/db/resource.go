package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/util"
)

var ErrPinnedThroughConfig = errors.New("resource is pinned through config")

const CheckBuildName = "check"

type CausalityDirection string

const (
	CausalityDownstream CausalityDirection = "downstream"
	CausalityUpstream   CausalityDirection = "upstream"

	downStreamCausalityQuery = `
WITH RECURSIVE build_ids AS (
		SELECT DISTINCT i.build_id
		FROM build_resource_config_version_inputs i
		WHERE i.resource_id=$1 AND i.version_md5=$2
	UNION
		SELECT i.build_id
		FROM build_ids bi
		INNER JOIN build_resource_config_version_outputs o ON o.build_id = bi.build_id
		INNER JOIN build_resource_config_version_inputs i ON i.resource_id = o.resource_id AND i.version_md5 = o.version_md5
		WHERE i.resource_id!=$1
)
`

	upStreamCausalityQuery = `
WITH RECURSIVE build_ids AS (
		SELECT DISTINCT o.build_id
		FROM build_resource_config_version_outputs o
		WHERE o.resource_id=$1 AND o.version_md5=$2
	UNION
		SELECT o.build_id
		FROM build_ids bi
		INNER JOIN build_resource_config_version_inputs i ON i.build_id = bi.build_id
		INNER JOIN build_resource_config_version_outputs o ON o.resource_id = i.resource_id AND o.version_md5 = i.version_md5
		WHERE i.resource_id!=$1
)
`
	// arbitrary numbers based around what we observed to be reasonable
	causalityMaxBuilds        = 5000
	causalityMaxInputsOutputs = 25000
)

var (
	ErrTooManyBuilds           = errors.New("too many builds")
	ErrTooManyResourceVersions = errors.New("too many resoruce versions")
)

//counterfeiter:generate . Resource
type Resource interface {
	PipelineRef

	ID() int
	Name() string
	Public() bool
	TeamID() int
	TeamName() string
	Type() string
	Source() atc.Source
	CheckEvery() *atc.CheckEvery
	CheckTimeout() string
	LastCheckStartTime() time.Time
	LastCheckEndTime() time.Time
	Tags() atc.Tags
	WebhookToken() string
	Config() atc.ResourceConfig
	ConfigPinnedVersion() atc.Version
	APIPinnedVersion() atc.Version
	PinComment() string
	SetPinComment(string) error
	ResourceConfigID() int
	ResourceConfigScopeID() int
	Icon() string

	HasWebhook() bool

	CurrentPinnedVersion() atc.Version

	BuildSummary() *atc.BuildSummary

	Versions(page Page, versionFilter atc.Version) ([]atc.ResourceVersion, Pagination, bool, error)
	FindVersion(filter atc.Version) (ResourceConfigVersion, bool, error) // Only used in tests!!
	UpdateMetadata(atc.Version, ResourceConfigMetadataFields) (bool, error)

	EnableVersion(rcvID int) error
	DisableVersion(rcvID int) error

	PinVersion(rcvID int) (bool, error)
	UnpinVersion() error

	Causality(rcvID int, direction CausalityDirection) (atc.Causality, bool, error)

	SetResourceConfigScope(ResourceConfigScope) error

	CheckPlan(atc.Version, time.Duration, ResourceTypes, atc.Source) atc.CheckPlan
	CreateBuild(context.Context, bool, atc.Plan) (Build, bool, error)
	CreateInMemoryBuild(context.Context, atc.Plan, util.SequenceGenerator) (Build, error)

	NotifyScan() error

	ClearResourceCache(atc.Version) (int64, error)

	Reload() (bool, error)
}

var (
	resourcesQuery = psql.Select(
		"r.id",
		"r.name",
		"r.type",
		"r.config",
		"rs.last_check_start_time",
		"rs.last_check_end_time",
		"rs.last_check_build_id",
		"rs.last_check_succeeded",
		"rs.last_check_build_plan",
		"r.pipeline_id",
		"r.nonce",
		"r.resource_config_id",
		"r.resource_config_scope_id",
		"p.name",
		"p.instance_vars",
		"t.id",
		"t.name",
		"rp.version",
		"rp.comment_text",
		"rp.config",
	).
		From("resources r").
		Join("pipelines p ON p.id = r.pipeline_id").
		Join("teams t ON t.id = p.team_id").
		LeftJoin("resource_config_scopes rs ON r.resource_config_scope_id = rs.id").
		LeftJoin("resource_pins rp ON rp.resource_id = r.id").
		Where(sq.Eq{"r.active": true})
)

type resource struct {
	pipelineRef

	id                    int
	name                  string
	teamID                int
	teamName              string
	type_                 string
	lastCheckStartTime    time.Time
	lastCheckEndTime      time.Time
	config                atc.ResourceConfig
	configPinnedVersion   atc.Version
	apiPinnedVersion      atc.Version
	pinComment            string
	resourceConfigID      int
	resourceConfigScopeID int
	buildSummary          *atc.BuildSummary
}

func newEmptyResource(conn Conn, lockFactory lock.LockFactory) *resource {
	return &resource{pipelineRef: pipelineRef{conn: conn, lockFactory: lockFactory}}
}

type ResourceNotFoundError struct {
	ID int
}

func (e ResourceNotFoundError) Error() string {
	return fmt.Sprintf("resource '%d' not found", e.ID)
}

type Resources []Resource

func (resources Resources) Lookup(name string) (Resource, bool) {
	for _, resource := range resources {
		if resource.Name() == name {
			return resource, true
		}
	}

	return nil, false
}

func (resources Resources) Configs() atc.ResourceConfigs {
	var configs atc.ResourceConfigs
	for _, r := range resources {
		configs = append(configs, r.Config())
	}
	return configs
}

func (r *resource) ID() int                          { return r.id }
func (r *resource) Name() string                     { return r.name }
func (r *resource) Public() bool                     { return r.config.Public }
func (r *resource) TeamID() int                      { return r.teamID }
func (r *resource) TeamName() string                 { return r.teamName }
func (r *resource) Type() string                     { return r.type_ }
func (r *resource) Source() atc.Source               { return r.config.Source }
func (r *resource) CheckEvery() *atc.CheckEvery      { return r.config.CheckEvery }
func (r *resource) CheckTimeout() string             { return r.config.CheckTimeout }
func (r *resource) LastCheckStartTime() time.Time    { return r.lastCheckStartTime }
func (r *resource) LastCheckEndTime() time.Time      { return r.lastCheckEndTime }
func (r *resource) Tags() atc.Tags                   { return r.config.Tags }
func (r *resource) WebhookToken() string             { return r.config.WebhookToken }
func (r *resource) Config() atc.ResourceConfig       { return r.config }
func (r *resource) ConfigPinnedVersion() atc.Version { return r.configPinnedVersion }
func (r *resource) APIPinnedVersion() atc.Version    { return r.apiPinnedVersion }
func (r *resource) PinComment() string               { return r.pinComment }
func (r *resource) ResourceConfigID() int            { return r.resourceConfigID }
func (r *resource) ResourceConfigScopeID() int       { return r.resourceConfigScopeID }
func (r *resource) Icon() string                     { return r.config.Icon }

func (r *resource) HasWebhook() bool { return r.WebhookToken() != "" }

func (r *resource) Reload() (bool, error) {
	row := resourcesQuery.Where(sq.Eq{"r.id": r.id}).
		RunWith(r.conn).
		QueryRow()

	err := scanResource(r, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (r *resource) SetResourceConfigScope(scope ResourceConfigScope) error {
	tx, err := r.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	err = r.setResourceConfigScopeInTransaction(tx, scope)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (r *resource) setResourceConfigScopeInTransaction(tx Tx, scope ResourceConfigScope) error {
	results, err := psql.Update("resources").
		Set("resource_config_id", scope.ResourceConfig().ID()).
		Set("resource_config_scope_id", scope.ID()).
		Where(sq.Eq{"id": r.id}).
		Where(sq.Or{
			sq.Eq{"resource_config_id": nil},
			sq.Eq{"resource_config_scope_id": nil},
			sq.NotEq{"resource_config_id": scope.ResourceConfig().ID()},
			sq.NotEq{"resource_config_scope_id": scope.ID()},
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected > 0 {
		err = requestScheduleForJobsUsingResource(tx, r.id)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *resource) CheckPlan(from atc.Version, interval time.Duration, resourceTypes ResourceTypes, sourceDefaults atc.Source) atc.CheckPlan {
	return atc.CheckPlan{
		Name:    r.Name(),
		Type:    r.Type(),
		Source:  sourceDefaults.Merge(r.Source()),
		Tags:    r.Tags(),
		Timeout: r.CheckTimeout(),

		FromVersion:            from,
		Interval:               interval.String(),
		VersionedResourceTypes: resourceTypes.Deserialize(),

		Resource: r.Name(),
	}
}

func (r *resource) CreateBuild(ctx context.Context, manuallyTriggered bool, plan atc.Plan) (Build, bool, error) {
	tx, err := r.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer Rollback(tx)

	if !manuallyTriggered {
		var numRunningBuilds int
		err = psql.Select("COUNT(1)").
			From("builds").
			Where(sq.Eq{"resource_id": r.id, "completed": false}).
			RunWith(tx).
			QueryRow().
			Scan(&numRunningBuilds)
		if err != nil {
			return nil, false, err
		}

		if numRunningBuilds > 0 {
			// a build is already running; leave it be
			return nil, false, nil
		}
	}

	build := newEmptyBuild(r.conn, r.lockFactory)
	err = createStartedBuild(tx, build, startedBuildArgs{
		Name:              CheckBuildName,
		PipelineID:        r.pipelineID,
		TeamID:            r.teamID,
		Plan:              plan,
		ManuallyTriggered: manuallyTriggered,
		SpanContext:       NewSpanContext(ctx),
		ExtraValues: map[string]interface{}{
			"resource_id": r.id,
		},
	})
	if err != nil {
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	err = r.conn.Bus().Notify(atc.ComponentBuildTracker)
	if err != nil {
		return nil, false, err
	}

	_, err = build.Reload()
	if err != nil {
		return nil, false, err
	}

	return build, true, nil
}

func (r *resource) CreateInMemoryBuild(ctx context.Context, plan atc.Plan, seqGen util.SequenceGenerator) (Build, error) {
	return newRunningInMemoryCheckBuild(r.conn, r.lockFactory, r, plan, NewSpanContext(ctx), seqGen)
}

func (r *resource) UpdateMetadata(version atc.Version, metadata ResourceConfigMetadataFields) (bool, error) {
	versionJSON, err := json.Marshal(version)
	if err != nil {
		return false, err
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return false, err
	}

	_, err = psql.Update("resource_config_versions").
		Set("metadata", string(metadataJSON)).
		Where(sq.Eq{
			"resource_config_scope_id": r.ResourceConfigScopeID(),
		}).
		Where(sq.Expr(
			"version_md5 = md5(?)", versionJSON,
		)).
		RunWith(r.conn).
		Exec()

	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// XXX: Deprecated, only used in tests
func (r *resource) FindVersion(v atc.Version) (ResourceConfigVersion, bool, error) {
	if r.resourceConfigScopeID == 0 {
		return nil, false, nil
	}

	ver := &resourceConfigVersion{
		conn: r.conn,
	}

	versionByte, err := json.Marshal(v)
	if err != nil {
		return nil, false, err
	}

	row := resourceConfigVersionQuery.
		Where(sq.Eq{
			"v.resource_config_scope_id": r.resourceConfigScopeID,
		}).
		Where(sq.Expr("v.version_md5 = md5(?)", versionByte)).
		RunWith(r.conn).
		QueryRow()

	err = scanResourceConfigVersion(ver, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return ver, true, nil
}

func (r *resource) SetPinComment(comment string) error {
	_, err := psql.Update("resource_pins").
		Set("comment_text", comment).
		Where(sq.Eq{"resource_id": r.ID()}).
		RunWith(r.conn).
		Exec()

	return err
}

func (r *resource) CurrentPinnedVersion() atc.Version {
	if r.configPinnedVersion != nil {
		return r.configPinnedVersion
	} else if r.apiPinnedVersion != nil {
		return r.apiPinnedVersion
	}
	return nil
}

func (r *resource) BuildSummary() *atc.BuildSummary {
	return r.buildSummary
}

func (r *resource) Versions(page Page, versionFilter atc.Version) ([]atc.ResourceVersion, Pagination, bool, error) {
	tx, err := r.conn.Begin()
	if err != nil {
		return nil, Pagination{}, false, err
	}

	defer Rollback(tx)

	query := `
		SELECT v.id, v.version, v.metadata, v.check_order,
			NOT EXISTS (
				SELECT 1
				FROM resource_disabled_versions d
				WHERE v.version_md5 = d.version_md5
				AND r.resource_config_scope_id = v.resource_config_scope_id
				AND r.id = d.resource_id
			)
		FROM resource_config_versions v, resources r
		WHERE r.id = $1 AND r.resource_config_scope_id = v.resource_config_scope_id
	`

	filterJSON := "{}"
	if len(versionFilter) != 0 {
		filterBytes, err := json.Marshal(versionFilter)
		if err != nil {
			return nil, Pagination{}, false, err
		}

		filterJSON = string(filterBytes)
	}

	var rows *sql.Rows
	if page.From != nil {
		rows, err = tx.Query(fmt.Sprintf(`
			SELECT sub.*
				FROM (
						%s
					AND version @> $4
					AND v.check_order >= (SELECT check_order FROM resource_config_versions WHERE id = $2)
				ORDER BY v.check_order ASC
				LIMIT $3
			) sub
			ORDER BY sub.check_order DESC
		`, query), r.id, *page.From, page.Limit, filterJSON)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else if page.To != nil {
		rows, err = tx.Query(fmt.Sprintf(`
			%s
				AND version @> $4
				AND v.check_order <= (SELECT check_order FROM resource_config_versions WHERE id = $2)
			ORDER BY v.check_order DESC
			LIMIT $3
		`, query), r.id, *page.To, page.Limit, filterJSON)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else {
		rows, err = tx.Query(fmt.Sprintf(`
			%s
			AND version @> $3
			ORDER BY v.check_order DESC
			LIMIT $2
		`, query), r.id, page.Limit, filterJSON)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	}

	defer Close(rows)

	type rcvCheckOrder struct {
		ResourceConfigVersionID int
		CheckOrder              int
	}

	rvs := make([]atc.ResourceVersion, 0)
	checkOrderRVs := make([]rcvCheckOrder, 0)
	for rows.Next() {
		var (
			metadataBytes sql.NullString
			versionBytes  string
			checkOrder    int
		)

		rv := atc.ResourceVersion{}
		err := rows.Scan(&rv.ID, &versionBytes, &metadataBytes, &checkOrder, &rv.Enabled)
		if err != nil {
			return nil, Pagination{}, false, err
		}

		err = json.Unmarshal([]byte(versionBytes), &rv.Version)
		if err != nil {
			return nil, Pagination{}, false, err
		}

		if metadataBytes.Valid {
			err = json.Unmarshal([]byte(metadataBytes.String), &rv.Metadata)
			if err != nil {
				return nil, Pagination{}, false, err
			}
		}

		checkOrderRV := rcvCheckOrder{
			ResourceConfigVersionID: rv.ID,
			CheckOrder:              checkOrder,
		}

		rvs = append(rvs, rv)
		checkOrderRVs = append(checkOrderRVs, checkOrderRV)
	}

	if len(rvs) == 0 {
		return nil, Pagination{}, true, nil
	}

	newestRCVCheckOrder := checkOrderRVs[0]
	oldestRCVCheckOrder := checkOrderRVs[len(checkOrderRVs)-1]

	var pagination Pagination

	var olderRCVId int
	err = tx.QueryRow(`
		SELECT v.id
		FROM resource_config_versions v, resources r
		WHERE v.check_order < $2 AND r.id = $1 AND v.resource_config_scope_id = r.resource_config_scope_id
		ORDER BY v.check_order DESC
		LIMIT 1
	`, r.id, oldestRCVCheckOrder.CheckOrder).Scan(&olderRCVId)
	if err != nil && err != sql.ErrNoRows {
		return nil, Pagination{}, false, err
	} else if err == nil {
		pagination.Older = &Page{
			To:    &olderRCVId,
			Limit: page.Limit,
		}
	}

	var newerRCVId int
	err = tx.QueryRow(`
		SELECT v.id
		FROM resource_config_versions v, resources r
		WHERE v.check_order > $2 AND r.id = $1 AND v.resource_config_scope_id = r.resource_config_scope_id
		ORDER BY v.check_order ASC
		LIMIT 1
	`, r.id, newestRCVCheckOrder.CheckOrder).Scan(&newerRCVId)
	if err != nil && err != sql.ErrNoRows {
		return nil, Pagination{}, false, err
	} else if err == nil {
		pagination.Newer = &Page{
			From:  &newerRCVId,
			Limit: page.Limit,
		}
	}

	err = tx.Commit()
	if err != nil {
		return nil, Pagination{}, false, nil
	}

	return rvs, pagination, true, nil
}

func (r *resource) EnableVersion(rcvID int) error {
	return r.toggleVersion(rcvID, true)
}

func (r *resource) DisableVersion(rcvID int) error {
	return r.toggleVersion(rcvID, false)
}

func (r *resource) PinVersion(rcvID int) (bool, error) {
	tx, err := r.conn.Begin()
	if err != nil {
		return false, err
	}
	defer Rollback(tx)
	var pinnedThroughConfig bool
	err = tx.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM resource_pins
			WHERE resource_id = $1
			AND config
		)`, r.id).Scan(&pinnedThroughConfig)
	if err != nil {
		return false, err
	}

	if pinnedThroughConfig {
		return false, ErrPinnedThroughConfig
	}

	results, err := tx.Exec(`
	    INSERT INTO resource_pins(resource_id, version, comment_text, config)
			VALUES ($1,
				( SELECT rcv.version
				FROM resource_config_versions rcv
				WHERE rcv.id = $2 ),
				'', false)
			ON CONFLICT (resource_id) DO UPDATE SET version=EXCLUDED.version`, r.id, rcvID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		return false, err
	}

	if rowsAffected != 1 {
		return false, nil
	}

	err = requestScheduleForJobsUsingResource(tx, r.id)
	if err != nil {
		return false, err
	}

	err = tx.Commit()
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *resource) UnpinVersion() error {
	tx, err := r.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	results, err := psql.Delete("resource_pins").
		Where(sq.Eq{"resource_pins.resource_id": r.id}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return NonOneRowAffectedError{rowsAffected}
	}

	err = requestScheduleForJobsUsingResource(tx, r.id)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (r *resource) toggleVersion(rcvID int, enable bool) error {
	tx, err := r.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	var results sql.Result
	if enable {
		results, err = tx.Exec(`
			DELETE FROM resource_disabled_versions
			WHERE resource_id = $1
			AND version_md5 = (SELECT version_md5 FROM resource_config_versions rcv WHERE rcv.id = $2)
			`, r.id, rcvID)
	} else {
		results, err = tx.Exec(`
			INSERT INTO resource_disabled_versions (resource_id, version_md5)
			SELECT $1, rcv.version_md5
			FROM resource_config_versions rcv
			WHERE rcv.id = $2
			`, r.id, rcvID)
	}
	if err != nil {
		return err
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return NonOneRowAffectedError{rowsAffected}
	}

	err = requestScheduleForJobsUsingResource(tx, r.id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *resource) NotifyScan() error {
	return r.conn.Bus().Notify(fmt.Sprintf("resource_scan_%d", r.id))
}

func (r *resource) ClearResourceCache(version atc.Version) (int64, error) {
	tx, err := r.conn.Begin()
	if err != nil {
		return 0, err
	}

	defer Rollback(tx)

	selectStatement := psql.Select("id").
		From("resource_caches").
		Where(sq.Eq{
			"resource_config_id": r.resourceConfigID,
		})

	if version != nil {
		versionJson, err := json.Marshal(version)
		if err != nil {
			return 0, err
		}

		selectStatement = selectStatement.Where(
			sq.Expr("version_md5 = md5(?)", versionJson),
		)
	}

	sqlStatement, args, err := selectStatement.ToSql()
	if err != nil {
		return 0, err
	}

	results, err := tx.Exec(`DELETE FROM worker_resource_caches WHERE resource_cache_id IN (`+sqlStatement+`)`, args...)

	if err != nil {
		return 0, err
	}

	rowsDeleted, err := results.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsDeleted, tx.Commit()
}

func scanResource(r *resource, row scannable) error {
	var (
		configBlob                                        sql.NullString
		nonce, rcID, rcScopeID, pinnedVersion, pinComment sql.NullString
		lastCheckStartTime, lastCheckEndTime              pq.NullTime
		lastCheckBuildId                                  sql.NullInt64
		lastCheckSucceeded                                sql.NullBool
		lastCheckBuildPlan                                sql.NullString
		pinnedThroughConfig                               sql.NullBool
		pipelineInstanceVars                              sql.NullString
	)

	err := row.Scan(&r.id, &r.name, &r.type_, &configBlob, &lastCheckStartTime,
		&lastCheckEndTime, &lastCheckBuildId, &lastCheckSucceeded, &lastCheckBuildPlan,
		&r.pipelineID, &nonce, &rcID, &rcScopeID,
		&r.pipelineName, &pipelineInstanceVars, &r.teamID, &r.teamName,
		&pinnedVersion, &pinComment, &pinnedThroughConfig)
	if err != nil {
		return err
	}

	r.lastCheckStartTime = lastCheckStartTime.Time
	r.lastCheckEndTime = lastCheckEndTime.Time

	es := r.conn.EncryptionStrategy()

	var noncense *string
	if nonce.Valid {
		noncense = &nonce.String
	}

	if configBlob.Valid {
		decryptedConfig, err := es.Decrypt(configBlob.String, noncense)
		if err != nil {
			return err
		}

		err = json.Unmarshal(decryptedConfig, &r.config)
		if err != nil {
			return err
		}
	} else {
		r.config = atc.ResourceConfig{}
	}

	if pinnedVersion.Valid {
		var version atc.Version
		err = json.Unmarshal([]byte(pinnedVersion.String), &version)
		if err != nil {
			return err
		}

		if pinnedThroughConfig.Valid && pinnedThroughConfig.Bool {
			r.configPinnedVersion = version
			r.apiPinnedVersion = nil
		} else {
			r.configPinnedVersion = nil
			r.apiPinnedVersion = version
		}
	} else {
		r.apiPinnedVersion = nil
		r.configPinnedVersion = nil
	}

	if pinComment.Valid {
		r.pinComment = pinComment.String
	} else {
		r.pinComment = ""
	}

	if rcID.Valid {
		r.resourceConfigID, err = strconv.Atoi(rcID.String)
		if err != nil {
			return err
		}
	}

	if rcScopeID.Valid {
		r.resourceConfigScopeID, err = strconv.Atoi(rcScopeID.String)
		if err != nil {
			return err
		}
	}

	if pipelineInstanceVars.Valid {
		err = json.Unmarshal([]byte(pipelineInstanceVars.String), &r.pipelineInstanceVars)
		if err != nil {
			return err
		}
	}

	if lastCheckBuildId.Valid {
		r.buildSummary = &atc.BuildSummary{
			ID:   int(lastCheckBuildId.Int64),
			Name: CheckBuildName,

			TeamName: r.teamName,

			PipelineID:           r.pipelineID,
			PipelineName:         r.pipelineName,
			PipelineInstanceVars: r.pipelineInstanceVars,
		}

		err := populateBuildSummary(r.buildSummary, lastCheckStartTime, lastCheckEndTime, lastCheckSucceeded, lastCheckBuildPlan)
		if err != nil {
			return err
		}
	}

	return nil
}

func populateBuildSummary(buildSummary *atc.BuildSummary,
	lastCheckStartTime, lastCheckEndTime pq.NullTime,
	lastCheckSucceeded sql.NullBool,
	lastCheckBuildPlan sql.NullString) error {
	if lastCheckStartTime.Valid {
		buildSummary.StartTime = lastCheckStartTime.Time.Unix()

		if lastCheckEndTime.Valid && lastCheckStartTime.Time.Before(lastCheckEndTime.Time) {
			buildSummary.EndTime = lastCheckEndTime.Time.Unix()
			if lastCheckSucceeded.Valid && lastCheckSucceeded.Bool {
				buildSummary.Status = atc.StatusSucceeded
			} else {
				buildSummary.Status = atc.StatusFailed
			}
		} else {
			buildSummary.Status = atc.StatusStarted
		}
	} else {
		buildSummary.Status = atc.StatusPending
	}

	if lastCheckBuildPlan.Valid {
		err := json.Unmarshal([]byte(lastCheckBuildPlan.String), &buildSummary.PublicPlan)
		if err != nil {
			return err
		}
	}

	return nil
}

// The SELECT query orders the jobs for updating to prevent deadlocking.
// Updating multiple rows using a SELECT subquery does not preserve the same
// order for the updates, which can lead to deadlocking.
func requestScheduleForJobsUsingResource(tx Tx, resourceID int) error {
	rows, err := psql.Select("DISTINCT job_id").
		From("job_inputs").
		Where(sq.Eq{
			"resource_id": resourceID,
		}).
		OrderBy("job_id DESC").
		RunWith(tx).
		Query()
	if err != nil {
		return err
	}

	var jobs []int
	for rows.Next() {
		var jid int
		err = rows.Scan(&jid)
		if err != nil {
			return err
		}

		jobs = append(jobs, jid)
	}

	for _, j := range jobs {
		_, err := psql.Update("jobs").
			Set("schedule_requested", sq.Expr("now()")).
			Where(sq.Eq{
				"id": j,
			}).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}
	}

	return nil
}

// this allows us to reuse getCausalityResourceVersions to construct both upstream and downstream trees by passing in a different updater fn
type resourceVersionUpdater func(*atc.CausalityResourceVersion, *atc.CausalityBuild)

func contains(lst []int, e int) bool {
	for _, v := range lst {
		if e == v {
			return true
		}
	}
	return false
}

func causalityQuery(direction CausalityDirection) string {
	switch direction {
	case CausalityDownstream:
		return downStreamCausalityQuery
	case CausalityUpstream:
		return upStreamCausalityQuery
	default:
		return ""
	}
}

type resourceVersionKey struct {
	resourceID int
	versionID  int
}

// causalityBuilds figures out all the builds that are related to a particular resource version
// This can include builds that were used the resource version (and its descendents) as an input,
// and builds that generated some ancestor of the build that generated the resource version itself.
func causalityBuilds(tx Tx, resourceID int, versionMD5 string, direction CausalityDirection) (map[int]*atc.CausalityBuild, map[int]*atc.CausalityJob, error) {
	query := causalityQuery(direction)
	// construct the job and build nodes. These are placed into a map for easy access down the line
	rows, err := psql.Select("b.id", "b.name", "b.status", "j.id", "j.name").
		Prefix(query, resourceID, versionMD5).
		From("build_ids bi").
		Join("builds b ON b.id = bi.build_id").
		Join("jobs j ON b.job_id = j.id").
		Limit(causalityMaxBuilds).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	builds := make(map[int]*atc.CausalityBuild, 0)
	jobs := make(map[int]*atc.CausalityJob, 0)

	var i int
	for i = 0; rows.Next(); i++ {
		var buildID, jobID int
		var buildName, jobName, status string

		err = rows.Scan(&buildID, &buildName, &status, &jobID, &jobName)
		if err != nil {
			return nil, nil, err
		}

		job, found := jobs[jobID]
		if !found {
			job = &atc.CausalityJob{
				ID:   jobID,
				Name: jobName,
			}
		}
		// since we only iterate over each build once, this should never insert any duplicates
		job.BuildIDs = append(job.BuildIDs, buildID)

		build := &atc.CausalityBuild{
			ID:     buildID,
			Name:   buildName,
			JobId:  jobID,
			Status: atc.BuildStatus(status),
		}

		builds[buildID] = build
		jobs[jobID] = job
	}
	if i == causalityMaxBuilds {
		return nil, nil, ErrTooManyBuilds
	}

	return builds, jobs, nil
}

func causalityResourceVersions(tx Tx, resourceID int, versionMD5 string, direction CausalityDirection, result *atc.Causality) error {
	// Bitmap to filter out resources that are not part of a output-input chain. This is done to filter out any other "root" resources.
	// For now, the causality view is only intersted in the "downstream" resources, not neccessarily "parallel input" resources
	// (reverse is true for upstream causality). If this ever changes in the future, it would be trivial to remove this filtering and get
	// a more "pipeline-like" causality view
	resourceHasParent := make(map[int]bool)
	resourceHasParent[resourceID] = true // the root resource is the only one that is not required to have a parent

	buildAsChild := func(rv *atc.CausalityResourceVersion, build *atc.CausalityBuild) {
		if !contains(rv.BuildIDs, build.ID) { // in case the same resource is used multiple times
			rv.BuildIDs = append(rv.BuildIDs, build.ID)
		}
	}
	resourceVersionAsChild := func(rv *atc.CausalityResourceVersion, build *atc.CausalityBuild) {
		if !contains(build.ResourceVersionIDs, rv.ID) { // in case a build outputs the same resource multiple times
			build.ResourceVersionIDs = append(build.ResourceVersionIDs, rv.ID)
		}

		resourceHasParent[rv.ResourceID] = true
	}

	var handleInput, handleOutput resourceVersionUpdater
	switch direction {
	case CausalityDownstream:
		// for every build input (resource version), add the build to its children
		// for every build output, register it as a child of the build
		handleInput = buildAsChild
		handleOutput = resourceVersionAsChild
	case CausalityUpstream:
		// for every build input (resource version), register it as a child of the build
		// for every build output, add the build to its children
		handleInput = resourceVersionAsChild
		handleOutput = buildAsChild
	}

	builds, jobs, err := causalityBuilds(tx, resourceID, versionMD5, direction)
	if err != nil {
		return err
	}

	query := causalityQuery(direction) + `
	SELECT r.id, rcv.id, r.name, rcv.version, i.build_id, 'input' AS type
	FROM build_resource_config_version_inputs i
	JOIN resources r ON r.id = i.resource_id
	JOIN resource_config_versions rcv ON rcv.version_md5 = i.version_md5 AND rcv.resource_config_scope_id = r.resource_config_scope_id
	JOIN build_ids bi ON i.build_id = bi.build_id
UNION ALL
	SELECT r.id, rcv.id, r.name, rcv.version, o.build_id, 'output' AS type
	FROM build_resource_config_version_outputs o
	JOIN resources r ON r.id = o.resource_id
	JOIN resource_config_versions rcv ON rcv.version_md5 = o.version_md5 AND rcv.resource_config_scope_id = r.resource_config_scope_id
	JOIN build_ids bi ON o.build_id = bi.build_id
LIMIT $3
`

	rows, err := tx.Query(query, resourceID, versionMD5, causalityMaxInputsOutputs)
	if err != nil {
		return err
	}

	defer rows.Close()

	resources := make(map[int]*atc.CausalityResource)
	resourceVersions := make(map[resourceVersionKey]*atc.CausalityResourceVersion)

	var i int
	for i = 0; rows.Next(); i++ {
		var (
			rID, rcvID, bID        int
			rName, versionStr, typ string
			version                atc.Version
		)
		err = rows.Scan(&rID, &rcvID, &rName, &versionStr, &bID, &typ)
		if err != nil {
			return err
		}

		err = json.Unmarshal([]byte(versionStr), &version)
		if err != nil {
			return err
		}

		r, found := resources[rID]
		if !found {
			r = &atc.CausalityResource{
				ID:   rID,
				Name: rName,
			}
		}
		if !contains(r.VersionIDs, rcvID) {
			r.VersionIDs = append(r.VersionIDs, rcvID)
		}

		rv, found := resourceVersions[resourceVersionKey{rID, rcvID}]
		if !found {
			rv = &atc.CausalityResourceVersion{
				ID:         rcvID,
				Version:    version,
				ResourceID: rID,
			}
		}

		switch typ {
		case "input":
			handleInput(rv, builds[bID])
		case "output":
			handleOutput(rv, builds[bID])
		default:
			return fmt.Errorf("unknown type: %v", typ)
		}

		resources[rID] = r
		resourceVersions[resourceVersionKey{rID, rcvID}] = rv
	}

	if i == causalityMaxInputsOutputs {
		return ErrTooManyResourceVersions
	}

	// flattening
	result.ResourceVersions = make([]atc.CausalityResourceVersion, 0, len(resourceVersions))
	unusedResourceVersions := make([]int, 0)
	for _, rv := range resourceVersions {
		if resourceHasParent[rv.ResourceID] {
			result.ResourceVersions = append(result.ResourceVersions, *rv)
		} else {
			unusedResourceVersions = append(unusedResourceVersions, rv.ID)
		}
	}
	result.Resources = make([]atc.CausalityResource, 0, len(resources))
	for id, r := range resources {
		if resourceHasParent[id] {
			result.Resources = append(result.Resources, *r)
		}
	}

	result.Builds = make([]atc.CausalityBuild, 0, len(builds))
	for _, b := range builds {
		// remove any resource version ids that don't have a parent
		filteredResourceVersions := make([]int, 0)
		for _, rvID := range b.ResourceVersionIDs {
			if !contains(unusedResourceVersions, rvID) {
				filteredResourceVersions = append(filteredResourceVersions, rvID)
			}
		}

		b.ResourceVersionIDs = filteredResourceVersions
		result.Builds = append(result.Builds, *b)
	}
	result.Jobs = make([]atc.CausalityJob, 0, len(jobs))
	for _, j := range jobs {
		result.Jobs = append(result.Jobs, *j)
	}
	return nil
}

func (r *resource) Causality(rcvID int, direction CausalityDirection) (atc.Causality, bool, error) {
	result := atc.Causality{}

	tx, err := r.conn.Begin() // in case some new build is started in between the 2 expensive queries
	if err != nil {
		return result, false, err
	}

	defer Rollback(tx) // everything is readonly, so no need to commit

	var versionMD5 string
	err = psql.Select("version_md5").
		From("resource_config_versions").
		Where(sq.Eq{"id": rcvID}).
		RunWith(tx).
		Scan(&versionMD5)
	if err != nil {
		return result, false, err
	}

	err = causalityResourceVersions(tx, r.id, versionMD5, direction, &result)
	if err != nil {
		return result, false, err
	}

	return result, true, nil
}
