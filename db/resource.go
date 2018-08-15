package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/lib/pq"
)

//go:generate counterfeiter . Resource

type Resource interface {
	ID() int
	Name() string
	PipelineName() string
	TeamName() string
	Type() string
	Source() atc.Source
	CheckEvery() string
	CheckTimeout() string
	LastChecked() time.Time
	Tags() atc.Tags
	CheckError() error
	Paused() bool
	WebhookToken() string
	PinnedVersion() atc.Version
	FailingToCheck() bool

	SetResourceConfig(int) error

	Pause() error
	Unpause() error

	Reload() (bool, error)
}

var resourcesQuery = psql.Select("r.id, r.name, r.config, r.check_error, r.paused, r.last_checked, r.pipeline_id, r.nonce, p.name, t.name").
	From("resources r").
	Join("pipelines p ON p.id = r.pipeline_id").
	Join("teams t ON t.id = p.team_id").
	Where(sq.Eq{"r.active": true})

type resource struct {
	id            int
	name          string
	pipelineID    int
	pipelineName  string
	teamName      string
	type_         string
	source        atc.Source
	checkEvery    string
	checkTimeout  string
	lastChecked   time.Time
	tags          atc.Tags
	checkError    error
	paused        bool
	webhookToken  string
	pinnedVersion atc.Version

	conn Conn
}

type ResourceNotFoundError struct {
	Name string
}

func (e ResourceNotFoundError) Error() string {
	return fmt.Sprintf("resource '%s' not found", e.Name)
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
			WebhookToken: r.WebhookToken(),
			Type:         r.Type(),
			Source:       r.Source(),
			CheckEvery:   r.CheckEvery(),
			Tags:         r.Tags(),
			Version:      r.PinnedVersion(),
		})
	}

	return configs
}

func (resources Resources) PinnedVersions() map[string]atc.Version {
	var pinnedVersions map[string]atc.Version
	for _, r := range resources {
		pinnedVersions[r.Name()] = r.PinnedVersion()
	}
	return pinnedVersions
}

func (r *resource) ID() int                    { return r.id }
func (r *resource) Name() string               { return r.name }
func (r *resource) PipelineID() int            { return r.pipelineID }
func (r *resource) PipelineName() string       { return r.pipelineName }
func (r *resource) TeamName() string           { return r.teamName }
func (r *resource) Type() string               { return r.type_ }
func (r *resource) Source() atc.Source         { return r.source }
func (r *resource) CheckEvery() string         { return r.checkEvery }
func (r *resource) CheckTimeout() string       { return r.checkTimeout }
func (r *resource) LastChecked() time.Time     { return r.lastChecked }
func (r *resource) Tags() atc.Tags             { return r.tags }
func (r *resource) CheckError() error          { return r.checkError }
func (r *resource) Paused() bool               { return r.paused }
func (r *resource) WebhookToken() string       { return r.webhookToken }
func (r *resource) PinnedVersion() atc.Version { return r.pinnedVersion }
func (r *resource) FailingToCheck() bool {
	return r.checkError != nil
}

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

func (r *resource) Pause() error {
	_, err := psql.Update("resources").
		Set("paused", true).
		Where(sq.Eq{
			"id": r.id,
		}).
		RunWith(r.conn).
		Exec()

	return err
}

func (r *resource) Unpause() error {
	_, err := psql.Update("resources").
		Set("paused", false).
		Where(sq.Eq{
			"id": r.id,
		}).
		RunWith(r.conn).
		Exec()

	return err
}

func (r *resource) SetResourceConfig(resourceConfigID int) error {
	_, err := psql.Update("resources").
		Set("resource_config_id", resourceConfigID).
		Where(sq.Eq{"id": r.id}).
		Where(sq.Or{
			sq.Eq{"resource_config_id": nil},
			sq.NotEq{"resource_config_id": resourceConfigID},
		}).
		RunWith(r.conn).
		Exec()

	return err
}

func scanResource(r *resource, row scannable) error {
	var (
		configBlob      []byte
		checkErr, nonce sql.NullString
		lastChecked     pq.NullTime
	)

	err := row.Scan(&r.id, &r.name, &configBlob, &checkErr, &r.paused, &lastChecked, &r.pipelineID, &nonce, &r.pipelineName, &r.teamName)
	if err != nil {
		return err
	}

	r.lastChecked = lastChecked.Time

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

	r.type_ = config.Type
	r.source = config.Source
	r.checkEvery = config.CheckEvery
	r.checkTimeout = config.CheckTimeout
	r.tags = config.Tags
	r.webhookToken = config.WebhookToken
	r.pinnedVersion = config.Version

	if checkErr.Valid {
		r.checkError = errors.New(checkErr.String)
	}

	return nil
}
