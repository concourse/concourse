package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
	multierror "github.com/hashicorp/go-multierror"
)

var ErrInvalidWebhookToken = errors.New("invalid webhook token")
var ErrMissingWebhook = errors.New("missing webhook")

func NewWebhooks(conn Conn, lockFactory lock.LockFactory, checkFactory CheckFactory) Webhooks {
	return Webhooks{
		conn:         conn,
		lockFactory:  lockFactory,
		checkFactory: checkFactory,
	}
}

type Webhooks struct {
	conn         Conn
	lockFactory  lock.LockFactory
	checkFactory CheckFactory
}

func (w Webhooks) CreateWebhook(teamID int, name string, type_ string, token string) error {
	es := w.conn.EncryptionStrategy()
	tokenEnc, nonce, err := es.Encrypt([]byte(token))
	if err != nil {
		return err
	}
	_, err = psql.Insert("webhooks").
		SetMap(map[string]interface{}{
			"name":    name,
			"team_id": teamID,
			"type":    type_,
			"token":   tokenEnc,
			"nonce":   nonce,
		}).
		RunWith(w.conn).
		Exec()
	if err != nil {
		return err
	}

	return err
}

func (w Webhooks) CheckResourcesMatchingWebhookPayload(logger lager.Logger, teamID int, name string, payload json.RawMessage, requestToken string) (int, error) {
	tx, err := w.conn.Begin()
	if err != nil {
		return 0, err
	}

	defer Rollback(tx)

	var (
		webhookType string
		tokenEnc    string
		nonce       sql.NullString
	)
	err = psql.Select("type", "token", "nonce").
		From("webhooks").
		Where(sq.Eq{
			"name":    name,
			"team_id": teamID,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&webhookType, &tokenEnc, &nonce)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrMissingWebhook
		}
		return 0, err
	}

	var token string
	if nonce.Valid {
		tokenDec, err := w.conn.EncryptionStrategy().Decrypt(tokenEnc, &nonce.String)
		if err != nil {
			return 0, err
		}
		token = string(tokenDec)
	} else {
		token = tokenEnc
	}

	if token != requestToken {
		return 0, ErrInvalidWebhookToken
	}

	rows, err := resourcesQuery.
		Join("resource_webhooks rw ON rw.resource_id = r.id").
		Where(sq.And{
			sq.Eq{
				"rw.webhook_type": webhookType,
				"p.team_id":       teamID,
				"p.paused":        false,
			},
			sq.Expr("?::jsonb @> rw.webhook_filter", string(payload)),
		}).
		RunWith(tx).
		Query()
	if err != nil {
		return 0, err
	}

	defer rows.Close()

	var resources []Resource
	for rows.Next() {
		resource := newEmptyResource(w.conn, w.lockFactory)
		if err := scanResource(resource, rows); err != nil {
			return 0, err
		}
		resources = append(resources, resource)
	}

	resourceTypesByPipeline := map[int]ResourceTypes{}
	for _, resource := range resources {
		pipelineID := resource.PipelineID()
		if _, exists := resourceTypesByPipeline[pipelineID]; !exists {
			resourceTypesByPipeline[pipelineID], err = w.pipelineResourceTypes(tx, pipelineID)
			if err != nil {
				return 0, err
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	var checkErrs error
	var numChecksCreated int
	for _, resource := range resources {
		logger := logger.Session("check-resource", lager.Data{"resource": resource.Name(), "pipeline": resource.PipelineID()})
		logger.Debug("matched")

		_, created, err := w.checkFactory.TryCreateCheck(
			lagerctx.NewContext(context.Background(), logger),
			resource,
			resourceTypesByPipeline[resource.PipelineID()],
			nil,
			true,
			false,
		)
		if err != nil {
			checkErrs = multierror.Append(checkErrs, err)
			continue
		}
		if !created {
			checkErrs = multierror.Append(checkErrs, errors.New("check not created"))
			continue
		}
		numChecksCreated++
	}

	return numChecksCreated, nil
}

func (w Webhooks) pipelineResourceTypes(tx Tx, pipelineID int) (ResourceTypes, error) {
	rows, err := resourceTypesQuery.
		Where(sq.Eq{"r.pipeline_id": pipelineID}).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var resourceTypes ResourceTypes
	for rows.Next() {
		resourceType := newEmptyResourceType(w.conn, w.lockFactory)
		if err := scanResourceType(resourceType, rows); err != nil {
			return nil, err
		}
		resourceTypes = append(resourceTypes, resourceType)
	}

	return resourceTypes, nil
}
