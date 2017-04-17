package dbng

import (
	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
)

//go:generate counterfeiter . ResourceConfigFactory

type ResourceConfigFactory interface {
	FindOrCreateResourceConfig(
		logger lager.Logger,
		user ResourceUser,
		resourceType string,
		source atc.Source,
		resourceTypes atc.VersionedResourceTypes,
	) (*UsedResourceConfig, error)

	CleanConfigUsesForFinishedBuilds() error
	CleanConfigUsesForInactiveResourceTypes() error
	CleanConfigUsesForInactiveResources() error
	CleanConfigUsesForPausedPipelinesResources() error
	CleanConfigUsesForOutdatedResourceConfigs() error
	CleanUselessConfigs() error

	AcquireResourceCheckingLock(
		logger lager.Logger,
		resourceUser ResourceUser,
		resourceType string,
		resourceSource atc.Source,
		resourceTypes atc.VersionedResourceTypes,
	) (lock.Lock, bool, error)
}

type resourceConfigFactory struct {
	conn        Conn
	lockFactory lock.LockFactory
}

func NewResourceConfigFactory(conn Conn, lockFactory lock.LockFactory) ResourceConfigFactory {
	return &resourceConfigFactory{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

func (f *resourceConfigFactory) FindOrCreateResourceConfig(
	logger lager.Logger,
	user ResourceUser,
	resourceType string,
	source atc.Source,
	resourceTypes atc.VersionedResourceTypes,
) (*UsedResourceConfig, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	resourceConfig, err := constructResourceConfig(tx, resourceType, source, resourceTypes)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	var usedResourceConfig *UsedResourceConfig

	err = safeFindOrCreate(f.conn, func(tx Tx) error {
		var err error

		usedResourceConfig, err = user.UseResourceConfig(logger, tx, f.lockFactory, resourceConfig)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return usedResourceConfig, nil
}

func constructResourceConfig(
	tx Tx,
	resourceType string,
	source atc.Source,
	resourceTypes atc.VersionedResourceTypes,
) (ResourceConfig, error) {
	resourceConfig := ResourceConfig{
		Source: source,
	}

	customType, found := resourceTypes.Lookup(resourceType)
	if found {
		customTypeResourceConfig, err := constructResourceConfig(
			tx,
			customType.Type,
			customType.Source,
			resourceTypes.Without(customType.Name),
		)
		if err != nil {
			return ResourceConfig{}, err
		}

		resourceConfig.CreatedByResourceCache = &ResourceCache{
			ResourceConfig: customTypeResourceConfig,
			Version:        customType.Version,
		}
	} else {
		resourceConfig.CreatedByBaseResourceType = &BaseResourceType{
			Name: resourceType,
		}
	}

	return resourceConfig, nil
}

func (f *resourceConfigFactory) CleanConfigUsesForFinishedBuilds() error {
	_, err := psql.Delete("resource_config_uses rcu USING builds b").
		Where(sq.Expr("rcu.build_id = b.id")).
		Where(sq.Expr("NOT b.interceptible")).
		RunWith(f.conn).
		Exec()

	if err != nil {
		return err
	}

	return nil
}

func (f *resourceConfigFactory) CleanConfigUsesForInactiveResourceTypes() error {
	_, err := psql.Delete("resource_config_uses rcu USING resource_types t").
		Where(sq.And{
			sq.Expr("rcu.resource_type_id = t.id"),
			sq.Eq{
				"t.active": false,
			},
		}).
		RunWith(f.conn).
		Exec()
	if err != nil {
		return err
	}

	return nil
}

func (f *resourceConfigFactory) CleanConfigUsesForInactiveResources() error {
	_, err := psql.Delete("resource_config_uses rcu USING resources r").
		Where(sq.And{
			sq.Expr("rcu.resource_id = r.id"),
			sq.Eq{
				"r.active": false,
			},
		}).
		RunWith(f.conn).
		Exec()
	if err != nil {
		return err
	}

	return nil
}

func (f *resourceConfigFactory) CleanConfigUsesForPausedPipelinesResources() error {
	pausedPipelineIds, _, err := sq.
		Select("id").
		Distinct().
		From("pipelines").
		Where(sq.Expr("paused = false")).
		ToSql()
	if err != nil {
		return err
	}

	_, err = psql.Delete("resource_config_uses rcu USING resources r").
		Where(sq.And{
			sq.Expr("r.pipeline_id NOT IN (" + pausedPipelineIds + ")"),
			sq.Expr("rcu.resource_id = r.id"),
		}).
		RunWith(f.conn).
		Exec()
	if err != nil {
		return err
	}

	return nil
}

func (f *resourceConfigFactory) CleanConfigUsesForOutdatedResourceConfigs() error {
	_, err := psql.Delete("resource_config_uses rcu USING resources r, resource_configs rc").
		Where(sq.And{
			sq.Expr("rcu.resource_id = r.id"),
			sq.Expr("rcu.resource_config_id = rc.id"),
			sq.Expr("r.source_hash != rc.source_hash"),
		}).
		RunWith(f.conn).
		Exec()
	if err != nil {
		return err
	}

	return nil
}

func (f *resourceConfigFactory) CleanUselessConfigs() error {
	stillInUseConfigIds, _, err := sq.
		Select("resource_config_id").
		Distinct().
		From("resource_config_uses").
		ToSql()
	if err != nil {
		return err
	}

	usedByResourceCachesIds, _, err := sq.
		Select("resource_config_id").
		Distinct().
		From("resource_caches").
		ToSql()
	if err != nil {
		return err
	}

	_, err = psql.Delete("resource_configs").
		Where("id NOT IN (" + stillInUseConfigIds + ")").
		Where("id NOT IN (" + usedByResourceCachesIds + ")").
		PlaceholderFormat(sq.Dollar).
		RunWith(f.conn).Exec()
	if err != nil {
		return err
	}

	return nil
}

func resourceTypesList(resourceTypeName string, allResourceTypes []atc.ResourceType, resultResourceTypes []atc.ResourceType) []atc.ResourceType {
	for _, resourceType := range allResourceTypes {
		if resourceType.Name == resourceTypeName {
			resultResourceTypes = append(resultResourceTypes, resourceType)
			return resourceTypesList(resourceType.Type, allResourceTypes, resultResourceTypes)
		}
	}

	return resultResourceTypes
}

func (f *resourceConfigFactory) AcquireResourceCheckingLock(
	logger lager.Logger,
	resourceUser ResourceUser,
	resourceType string,
	resourceSource atc.Source,
	resourceTypes atc.VersionedResourceTypes,
) (lock.Lock, bool, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	resourceConfig, err := constructResourceConfig(tx, resourceType, resourceSource, resourceTypes)
	if err != nil {
		return nil, false, err
	}

	return acquireResourceCheckingLock(
		logger.Session("lock", lager.Data{"resource-user": resourceUser}),
		tx,
		resourceUser,
		resourceConfig,
		f.lockFactory,
	)
}

func acquireResourceCheckingLock(
	logger lager.Logger,
	tx Tx,
	resourceUser ResourceUser,
	resourceConfig ResourceConfig,
	lockFactory lock.LockFactory,
) (lock.Lock, bool, error) {
	usedResourceConfig, err := resourceUser.UseResourceConfig(
		logger,
		tx,
		lockFactory,
		resourceConfig,
	)
	if err != nil {
		return nil, false, err
	}

	lock := lockFactory.NewLock(
		logger,
		lock.NewResourceConfigCheckingLockID(usedResourceConfig.ID),
	)

	acquired, err := lock.Acquire()
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	err = tx.Commit()
	if err != nil {
		lock.Release()
		return nil, false, err
	}

	return lock, true, nil
}
