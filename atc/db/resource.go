package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/lib/pq"
)

//go:generate counterfeiter . Resource

type Resource interface {
	ID() int
	Name() string
	Public() bool
	PipelineID() int
	PipelineName() string
	TeamID() int
	TeamName() string
	Type() string
	Source() atc.Source
	CheckEvery() string
	CheckTimeout() string
	LastCheckStartTime() time.Time
	LastCheckEndTime() time.Time
	Tags() atc.Tags
	CheckSetupError() error
	CheckError() error
	WebhookToken() string
	ConfigPinnedVersion() atc.Version
	APIPinnedVersion() atc.Version
	PinComment() string
	SetPinComment(string) error
	ResourceConfigID() int
	ResourceConfigScopeID() int
	Icon() string

	CurrentPinnedVersion() atc.Version

	ResourceConfigVersionID(atc.Version) (int, bool, error)
	Versions(page Page, versionFilter atc.Version) ([]atc.ResourceVersion, Pagination, bool, error)
	SaveUncheckedVersion(atc.Version, ResourceConfigMetadataFields, ResourceConfig, atc.VersionedResourceTypes) (bool, error)
	UpdateMetadata(atc.Version, ResourceConfigMetadataFields) (bool, error)

	EnableVersion(rcvID int) error
	DisableVersion(rcvID int) error

	PinVersion(rcvID int) (bool, error)
	UnpinVersion() error

	SetResourceConfig(atc.Source, atc.VersionedResourceTypes) (ResourceConfigScope, error)
	SetCheckSetupError(error) error
	NotifyScan() error

	Reload() (bool, error)
}

var resourcesQuery = psql.Select(
	"r.id",
	"r.name",
	"r.type",
	"r.config",
	"r.check_error",
	"rs.last_check_start_time",
	"rs.last_check_end_time",
	"r.pipeline_id",
	"r.nonce",
	"r.resource_config_id",
	"r.resource_config_scope_id",
	"p.name",
	"t.id",
	"t.name",
	"rs.check_error",
	"rp.version",
	"rp.comment_text",
).
	From("resources r").
	Join("pipelines p ON p.id = r.pipeline_id").
	Join("teams t ON t.id = p.team_id").
	LeftJoin("resource_config_scopes rs ON r.resource_config_scope_id = rs.id").
	LeftJoin("resource_pins rp ON rp.resource_id = r.id").
	Where(sq.Eq{"r.active": true})

type resource struct {
	id                    int
	name                  string
	public                bool
	pipelineID            int
	pipelineName          string
	teamID                int
	teamName              string
	type_                 string
	source                atc.Source
	checkEvery            string
	checkTimeout          string
	lastCheckStartTime    time.Time
	lastCheckEndTime      time.Time
	tags                  atc.Tags
	checkSetupError       error
	checkError            error
	webhookToken          string
	configPinnedVersion   atc.Version
	apiPinnedVersion      atc.Version
	pinComment            string
	resourceConfigID      int
	resourceConfigScopeID int
	icon                  string

	conn        Conn
	lockFactory lock.LockFactory
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
		configs = append(configs, atc.ResourceConfig{
			Name:         r.Name(),
			Public:       r.Public(),
			WebhookToken: r.WebhookToken(),
			Type:         r.Type(),
			Source:       r.Source(),
			CheckEvery:   r.CheckEvery(),
			Tags:         r.Tags(),
			Version:      r.ConfigPinnedVersion(),
			Icon:         r.Icon(),
		})
	}

	return configs
}

func (r *resource) ID() int                          { return r.id }
func (r *resource) Name() string                     { return r.name }
func (r *resource) Public() bool                     { return r.public }
func (r *resource) PipelineID() int                  { return r.pipelineID }
func (r *resource) PipelineName() string             { return r.pipelineName }
func (r *resource) TeamID() int                      { return r.teamID }
func (r *resource) TeamName() string                 { return r.teamName }
func (r *resource) Type() string                     { return r.type_ }
func (r *resource) Source() atc.Source               { return r.source }
func (r *resource) CheckEvery() string               { return r.checkEvery }
func (r *resource) CheckTimeout() string             { return r.checkTimeout }
func (r *resource) LastCheckStartTime() time.Time    { return r.lastCheckStartTime }
func (r *resource) LastCheckEndTime() time.Time      { return r.lastCheckEndTime }
func (r *resource) Tags() atc.Tags                   { return r.tags }
func (r *resource) CheckSetupError() error           { return r.checkSetupError }
func (r *resource) CheckError() error                { return r.checkError }
func (r *resource) WebhookToken() string             { return r.webhookToken }
func (r *resource) ConfigPinnedVersion() atc.Version { return r.configPinnedVersion }
func (r *resource) APIPinnedVersion() atc.Version    { return r.apiPinnedVersion }
func (r *resource) PinComment() string               { return r.pinComment }
func (r *resource) ResourceConfigID() int            { return r.resourceConfigID }
func (r *resource) ResourceConfigScopeID() int       { return r.resourceConfigScopeID }
func (r *resource) Icon() string                     { return r.icon }

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

func (r *resource) SetResourceConfig(source atc.Source, resourceTypes atc.VersionedResourceTypes) (ResourceConfigScope, error) {
	resourceConfigDescriptor, err := constructResourceConfigDescriptor(r.type_, source, resourceTypes)
	if err != nil {
		return nil, err
	}

	tx, err := r.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	resourceConfig, err := resourceConfigDescriptor.findOrCreate(tx, r.lockFactory, r.conn)
	if err != nil {
		return nil, err
	}

	_, err = psql.Update("resources").
		Set("resource_config_id", resourceConfig.ID()).
		Where(sq.Eq{"id": r.id}).
		Where(sq.Or{
			sq.Eq{"resource_config_id": nil},
			sq.NotEq{"resource_config_id": resourceConfig.ID()},
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, err
	}

	resourceConfigScope, err := findOrCreateResourceConfigScope(tx, r.conn, r.lockFactory, resourceConfig, r, resourceTypes)
	if err != nil {
		return nil, err
	}

	results, err := psql.Update("resources").
		Set("resource_config_scope_id", resourceConfigScope.ID()).
		Where(sq.Eq{"id": r.id}).
		Where(sq.Or{
			sq.Eq{"resource_config_scope_id": nil},
			sq.NotEq{"resource_config_scope_id": resourceConfigScope.ID()},
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, err
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	if rowsAffected > 0 {
		err = bumpCacheIndexForPipelinesUsingResourceConfigScope(r.conn, resourceConfigScope.ID())
		if err != nil {
			return nil, err
		}
	}

	return resourceConfigScope, nil
}

func (r *resource) SetCheckSetupError(cause error) error {
	var err error

	if cause == nil {
		_, err = psql.Update("resources").
			Set("check_error", nil).
			Where(sq.Eq{"id": r.ID()}).
			RunWith(r.conn).
			Exec()
	} else {
		_, err = psql.Update("resources").
			Set("check_error", cause.Error()).
			Where(sq.Eq{"id": r.ID()}).
			RunWith(r.conn).
			Exec()
	}

	return err
}

// XXX: only used for tests
func (r *resource) SaveUncheckedVersion(version atc.Version, metadata ResourceConfigMetadataFields, resourceConfig ResourceConfig, resourceTypes atc.VersionedResourceTypes) (bool, error) {
	tx, err := r.conn.Begin()
	if err != nil {
		return false, err
	}

	defer Rollback(tx)

	resourceConfigScope, err := findOrCreateResourceConfigScope(tx, r.conn, r.lockFactory, resourceConfig, r, resourceTypes)
	if err != nil {
		return false, err
	}

	newVersion, err := saveResourceVersion(tx, resourceConfigScope.ID(), version, metadata)
	if err != nil {
		return false, err
	}

	return newVersion, tx.Commit()
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

func (r *resource) ResourceConfigVersionID(version atc.Version) (int, bool, error) {
	requestedVersion, err := json.Marshal(version)
	if err != nil {
		return 0, false, err
	}

	var id int

	err = psql.Select("rcv.id").
		From("resource_config_versions rcv").
		Join("resources r ON rcv.resource_config_scope_id = r.resource_config_scope_id").
		Where(sq.Eq{"r.id": r.ID()}).
		Where(sq.Expr("version @> ?", requestedVersion)).
		Where(sq.NotEq{"rcv.check_order": 0}).
		OrderBy("rcv.check_order DESC").
		RunWith(r.conn).
		QueryRow().
		Scan(&id)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}

	return id, true, nil
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

func (r *resource) Versions(page Page, versionFilter atc.Version) ([]atc.ResourceVersion, Pagination, bool, error) {
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
		WHERE r.id = $1 AND r.resource_config_scope_id = v.resource_config_scope_id AND v.check_order != 0
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
	var err error
	if page.Until != 0 {
		rows, err = r.conn.Query(fmt.Sprintf(`
			SELECT sub.*
				FROM (
						%s
					AND version @> $4
					AND v.check_order > (SELECT check_order FROM resource_config_versions WHERE id = $2)
				ORDER BY v.check_order ASC
				LIMIT $3
			) sub
			ORDER BY sub.check_order DESC
		`, query), r.id, page.Until, page.Limit, filterJSON)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else if page.Since != 0 {
		rows, err = r.conn.Query(fmt.Sprintf(`
			%s
				AND version @> $4
				AND v.check_order < (SELECT check_order FROM resource_config_versions WHERE id = $2)
			ORDER BY v.check_order DESC
			LIMIT $3
		`, query), r.id, page.Since, page.Limit, filterJSON)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else if page.To != 0 {
		rows, err = r.conn.Query(fmt.Sprintf(`
			SELECT sub.*
				FROM (
						%s
					AND version @> $4
					AND v.check_order >= (SELECT check_order FROM resource_config_versions WHERE id = $2)
				ORDER BY v.check_order ASC
				LIMIT $3
			) sub
			ORDER BY sub.check_order DESC
		`, query), r.id, page.To, page.Limit, filterJSON)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else if page.From != 0 {
		rows, err = r.conn.Query(fmt.Sprintf(`
			%s
				AND version @> $4
				AND v.check_order <= (SELECT check_order FROM resource_config_versions WHERE id = $2)
			ORDER BY v.check_order DESC
			LIMIT $3
		`, query), r.id, page.From, page.Limit, filterJSON)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else {
		rows, err = r.conn.Query(fmt.Sprintf(`
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

	var minCheckOrder int
	var maxCheckOrder int

	err = r.conn.QueryRow(`
		SELECT COALESCE(MAX(v.check_order), 0) as maxCheckOrder,
			COALESCE(MIN(v.check_order), 0) as minCheckOrder
		FROM resource_config_versions v, resources r
		WHERE r.id = $1 AND v.resource_config_scope_id = r.resource_config_scope_id
	`, r.id).Scan(&maxCheckOrder, &minCheckOrder)
	if err != nil {
		return nil, Pagination{}, false, err
	}

	firstRCVCheckOrder := checkOrderRVs[0]
	lastRCVCheckOrder := checkOrderRVs[len(checkOrderRVs)-1]

	var pagination Pagination

	if firstRCVCheckOrder.CheckOrder < maxCheckOrder {
		pagination.Previous = &Page{
			Until: firstRCVCheckOrder.ResourceConfigVersionID,
			Limit: page.Limit,
		}
	}

	if lastRCVCheckOrder.CheckOrder > minCheckOrder {
		pagination.Next = &Page{
			Since: lastRCVCheckOrder.ResourceConfigVersionID,
			Limit: page.Limit,
		}
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
	results, err := r.conn.Exec(`
	    INSERT INTO resource_pins(resource_id, version, comment_text)
			VALUES ($1,
				( SELECT rcv.version
				FROM resource_config_versions rcv
				WHERE rcv.id = $2 ),
				'')
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

	return true, nil
}

func (r *resource) UnpinVersion() error {
	results, err := psql.Delete("resource_pins").
		Where(sq.Eq{"resource_pins.resource_id": r.id}).
		RunWith(r.conn).
		Exec()
	if err != nil {
		return err
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return nonOneRowAffectedError{rowsAffected}
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
		return nonOneRowAffectedError{rowsAffected}
	}

	err = bumpCacheIndex(tx, r.pipelineID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *resource) NotifyScan() error {
	return r.conn.Bus().Notify(fmt.Sprintf("resource_scan_%d", r.id))
}

func scanResource(r *resource, row scannable) error {
	var (
		configBlob                                                                  []byte
		checkErr, rcsCheckErr, nonce, rcID, rcScopeID, apiPinnedVersion, pinComment sql.NullString
		lastCheckStartTime, lastCheckEndTime                                        pq.NullTime
	)

	err := row.Scan(&r.id, &r.name, &r.type_, &configBlob, &checkErr, &lastCheckStartTime, &lastCheckEndTime, &r.pipelineID, &nonce, &rcID, &rcScopeID, &r.pipelineName, &r.teamID, &r.teamName, &rcsCheckErr, &apiPinnedVersion, &pinComment)
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

	decryptedConfig, err := es.Decrypt(string(configBlob), noncense)
	if err != nil {
		return err
	}

	var config atc.ResourceConfig
	err = json.Unmarshal(decryptedConfig, &config)
	if err != nil {
		return err
	}

	r.public = config.Public
	r.source = config.Source
	r.checkEvery = config.CheckEvery
	r.checkTimeout = config.CheckTimeout
	r.tags = config.Tags
	r.webhookToken = config.WebhookToken
	r.configPinnedVersion = config.Version
	r.icon = config.Icon

	if apiPinnedVersion.Valid {
		err = json.Unmarshal([]byte(apiPinnedVersion.String), &r.apiPinnedVersion)
		if err != nil {
			return err
		}
	} else {
		r.apiPinnedVersion = nil
	}

	if pinComment.Valid {
		r.pinComment = pinComment.String
	} else {
		r.pinComment = ""
	}

	if checkErr.Valid {
		r.checkSetupError = errors.New(checkErr.String)
	} else {
		r.checkSetupError = nil
	}

	if rcsCheckErr.Valid {
		r.checkError = errors.New(rcsCheckErr.String)
	} else {
		r.checkError = nil
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

	return nil
}
