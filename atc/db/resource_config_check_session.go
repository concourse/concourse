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
	ResourceConfig() ResourceConfig
}

type resourceConfigCheckSession struct {
	id             int
	resourceConfig ResourceConfig
}

func (session resourceConfigCheckSession) ID() int {
	return session.id
}

func (session resourceConfigCheckSession) ResourceConfig() ResourceConfig {
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
	resourceConfigDescriptor, err := constructResourceConfigDescriptor(resourceType, source, resourceTypes)
	if err != nil {
		return nil, err
	}

	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	resourceConfig, err := resourceConfigDescriptor.findOrCreate(logger, tx, factory.lockFactory, factory.conn)
	if err != nil {
		return nil, err
	}

	var rccsID int
	err = psql.Select("id").
		From("resource_config_check_sessions").
		Where(sq.And{
			sq.Eq{
				"resource_config_id": resourceConfig.ID(),
			},
			sq.Expr(fmt.Sprintf("expires_at > NOW() + interval '%d seconds'", int(expiries.GraceTime.Seconds()))),
		}).
		RunWith(tx).
		QueryRow().
		Scan(&rccsID)
	if err == sql.ErrNoRows {
		expiryStmt := fmt.Sprintf(
			"NOW() + LEAST(GREATEST('%d seconds'::interval, NOW() - to_timestamp(max(start_time))), '%d seconds'::interval)",
			int(expiries.Min.Seconds()),
			int(expiries.Max.Seconds()),
		)

		err = psql.Insert("resource_config_check_sessions").
			SetMap(map[string]interface{}{
				"resource_config_id": resourceConfig.ID(),
				"expires_at":         sq.Expr("(SELECT " + expiryStmt + " FROM workers)"),
			}).
			Suffix("RETURNING id").
			RunWith(tx).
			QueryRow().
			Scan(&rccsID)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return resourceConfigCheckSession{
		id:             rccsID,
		resourceConfig: resourceConfig,
	}, nil
}
