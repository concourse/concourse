package dbng

import (
	"fmt"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
	"github.com/lib/pq"
)

type ErrCustomResourceTypeVersionNotFound struct {
	Name string
}

func (e ErrCustomResourceTypeVersionNotFound) Error() string {
	return fmt.Sprintf("custom resource type '%s' version not found", e.Name)
}

//go:generate counterfeiter . ResourceConfigFactory

type ResourceConfigFactory interface {
	FindOrCreateResourceConfig(
		logger lager.Logger,
		user ResourceUser,
		resourceType string,
		source atc.Source,
		resourceTypes atc.VersionedResourceTypes,
	) (*UsedResourceConfig, error)

	FindResourceConfig(
		logger lager.Logger,
		resourceType string,
		source atc.Source,
		resourceTypes atc.VersionedResourceTypes,
	) (*UsedResourceConfig, bool, error)

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

func (f *resourceConfigFactory) FindResourceConfig(
	logger lager.Logger,
	resourceType string,
	source atc.Source,
	resourceTypes atc.VersionedResourceTypes,
) (*UsedResourceConfig, bool, error) {
	resourceConfig, err := constructResourceConfig(resourceType, source, resourceTypes)
	if err != nil {
		return nil, false, err
	}

	var usedResourceConfig *UsedResourceConfig

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()

	usedResourceConfig, found, err := resourceConfig.Find(logger, tx)
	if err != nil {
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return usedResourceConfig, true, nil
}

func (f *resourceConfigFactory) FindOrCreateResourceConfig(
	logger lager.Logger,
	user ResourceUser,
	resourceType string,
	source atc.Source,
	resourceTypes atc.VersionedResourceTypes,
) (*UsedResourceConfig, error) {
	resourceConfig, err := constructResourceConfig(resourceType, source, resourceTypes)
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

// constructResourceConfig cannot be called for constructing a resource type's
// resource config while also containing the same resource type in the list of
// resource types, because that results in a circular dependency.
func constructResourceConfig(
	resourceTypeName string,
	source atc.Source,
	resourceTypes atc.VersionedResourceTypes,
) (ResourceConfig, error) {
	resourceConfig := ResourceConfig{
		Source: source,
	}

	customType, found := resourceTypes.Lookup(resourceTypeName)
	if found {
		customTypeResourceConfig, err := constructResourceConfig(
			customType.Type,
			customType.Source,
			resourceTypes.Without(customType.Name),
		)
		if err != nil {
			return ResourceConfig{}, err
		}

		if customType.Version == nil {
			return ResourceConfig{}, ErrCustomResourceTypeVersionNotFound{Name: customType.Name}
		}

		resourceConfig.CreatedByResourceCache = &ResourceCache{
			ResourceConfig: customTypeResourceConfig,
			Version:        customType.Version,
		}
	} else {
		resourceConfig.CreatedByBaseResourceType = &BaseResourceType{
			Name: resourceTypeName,
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
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "foreign_key_violation" {
			// this can happen if a use or resource cache is created referencing the
			// config; as the subqueries above are not atomic
			return nil
		}

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
	resourceConfig, err := constructResourceConfig(resourceType, resourceSource, resourceTypes)
	if err != nil {
		return nil, false, err
	}

	logger.Debug("acquiring-resource-checking-lock", lager.Data{
		"resource-config": resourceConfig,
		"resource-type":   resourceType,
		"resource-source": resourceSource,
		"resource-types":  resourceTypes,
	})

	return acquireResourceCheckingLock(
		logger.Session("lock", lager.Data{"resource-user": resourceUser}),
		f.conn,
		resourceUser,
		resourceConfig,
		f.lockFactory,
	)
}

func acquireResourceCheckingLock(
	logger lager.Logger,
	conn Conn,
	user ResourceUser,
	resourceConfig ResourceConfig,
	lockFactory lock.LockFactory,
) (lock.Lock, bool, error) {
	var usedResourceConfig *UsedResourceConfig

	err := safeFindOrCreate(conn, func(tx Tx) error {
		var err error

		usedResourceConfig, err = user.UseResourceConfig(logger, tx, lockFactory, resourceConfig)
		if err != nil {
			return err
		}

		return nil
	})

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

	return lock, true, nil
}
