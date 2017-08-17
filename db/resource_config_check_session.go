package db

import (
	"database/sql"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db/lock"
)

//go:generate counterfeiter . ResourceConfigCheckSession

type ResourceConfigCheckSession interface {
	ID() int
	ResourceConfig() *UsedResourceConfig
}

type resourceConfigCheckSession struct {
	id             int
	resourceConfig *UsedResourceConfig
}

func (session resourceConfigCheckSession) ID() int {
	return session.id
}

func (session resourceConfigCheckSession) ResourceConfig() *UsedResourceConfig {
	return session.resourceConfig
}

//go:generate counterfeiter . ResourceConfigCheckSessionFactory

type ResourceConfigCheckSessionFactory interface {
	FindOrCreateResourceConfigCheckSession(
		logger lager.Logger,
		resourceType string,
		source atc.Source,
		resourceTypes creds.VersionedResourceTypes,
		expiries ContainerOwnerExpiries,
	) (ResourceConfigCheckSession, error)
}

type resourceConfigCheckSessionFactory struct {
	conn        Conn
	lockFactory lock.LockFactory
}

type ContainerOwnerExpiries struct {
	GraceTime time.Duration
	Min       time.Duration
	Max       time.Duration
}

func NewResourceConfigCheckSessionFactory(conn Conn, lockFactory lock.LockFactory) ResourceConfigCheckSessionFactory {
	return &resourceConfigCheckSessionFactory{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

func (factory resourceConfigCheckSessionFactory) FindOrCreateResourceConfigCheckSession(
	logger lager.Logger,
	resourceType string,
	source atc.Source,
	resourceTypes creds.VersionedResourceTypes,
	expiries ContainerOwnerExpiries,
) (ResourceConfigCheckSession, error) {
	resourceConfig, err := constructResourceConfig(resourceType, source, resourceTypes)
	if err != nil {
		return nil, err
	}

	var rccsID int
	var usedResourceConfig *UsedResourceConfig

	err = safeFindOrCreate(factory.conn, func(tx Tx) error {
		var err error

		usedResourceConfig, err = resourceConfig.findOrCreate(logger, tx)
		if err != nil {
			return err
		}

		err = psql.Select("id").
			From("resource_config_check_sessions").
			Where(sq.And{
				sq.Eq{
					"resource_config_id": usedResourceConfig.ID,
				},
				sq.Expr(fmt.Sprintf("expires_at > NOW() + interval '%d seconds'", int(expiries.GraceTime.Seconds()))),
			}).
			RunWith(tx).
			QueryRow().
			Scan(&rccsID)
		if err != nil {
			if err == sql.ErrNoRows {
				expiryStmt := fmt.Sprintf(
					"NOW() + LEAST(GREATEST('%d seconds'::interval, NOW() - to_timestamp(max(start_time))), '%d seconds'::interval)",
					int(expiries.Min.Seconds()),
					int(expiries.Max.Seconds()),
				)

				err = psql.Insert("resource_config_check_sessions").
					SetMap(map[string]interface{}{
						"resource_config_id": usedResourceConfig.ID,
						"expires_at":         sq.Expr("(SELECT " + expiryStmt + " FROM workers)"),
					}).
					Suffix("RETURNING id").
					RunWith(tx).
					QueryRow().
					Scan(&rccsID)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return resourceConfigCheckSession{
		id:             rccsID,
		resourceConfig: usedResourceConfig,
	}, nil
}
