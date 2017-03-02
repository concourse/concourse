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
		pipelineID int,
		resourceTypes atc.ResourceTypes,
	) (*UsedResourceConfig, error)

	CleanConfigUsesForFinishedBuilds() error
	CleanConfigUsesForInactiveResourceTypes() error
	CleanConfigUsesForInactiveResources() error
	CleanUselessConfigs() error
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
	pipelineID int,
	resourceTypes atc.ResourceTypes,
) (*UsedResourceConfig, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	resourceConfig, err := constructResourceConfig(tx, resourceType, source, resourceTypes, pipelineID)
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
	resourceTypes []atc.ResourceType,
	pipelineID int,
) (ResourceConfig, error) {
	resourceConfig := ResourceConfig{
		Source: source,
	}

	resourceTypesList := resourceTypesList(resourceType, resourceTypes, []atc.ResourceType{})
	if len(resourceTypesList) == 0 {
		resourceConfig.CreatedByBaseResourceType = &BaseResourceType{
			Name: resourceType,
		}
	} else {
		lastResourceType := resourceTypesList[len(resourceTypesList)-1]
		urt, found, err := ResourceType{
			ResourceType: lastResourceType,
			PipelineID:   pipelineID,
		}.Find(tx)
		if err != nil {
			return ResourceConfig{}, err
		}

		if !found {
			return ResourceConfig{}, ErrResourceTypeNotFound{lastResourceType.Name}
		}

		parentResourceCache := &ResourceCache{
			ResourceConfig: ResourceConfig{
				CreatedByBaseResourceType: &BaseResourceType{
					Name: lastResourceType.Type,
				},
				Source: lastResourceType.Source,
			},
			Version: urt.Version,
		}

		for i := len(resourceTypesList) - 2; i >= 0; i-- {
			urt, found, err := ResourceType{
				ResourceType: resourceTypesList[i],
				PipelineID:   pipelineID,
			}.Find(tx)
			if err != nil {
				return ResourceConfig{}, err
			}
			if !found {
				return ResourceConfig{}, ErrResourceTypeNotFound{resourceTypesList[i].Name}
			}

			parentResourceCache = &ResourceCache{
				ResourceConfig: ResourceConfig{
					CreatedByResourceCache: parentResourceCache,
					Source:                 resourceTypesList[i].Source,
				},
				Version: urt.Version,
			}
		}

		resourceConfig.CreatedByResourceCache = parentResourceCache
	}

	return resourceConfig, nil
}

func (f *resourceConfigFactory) CleanConfigUsesForFinishedBuilds() error {
	tx, err := f.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = psql.Delete("resource_config_uses rcu USING builds b").
		Where(sq.Expr("rcu.build_id = b.id")).
		Where(sq.Expr("NOT b.interceptible")).
		RunWith(tx).
		Exec()

	if err != nil {
		return err
	}

	return tx.Commit()
}

func (f *resourceConfigFactory) CleanConfigUsesForInactiveResourceTypes() error {
	tx, err := f.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = psql.Delete("resource_config_uses rcu USING resource_types t").
		Where(sq.And{
			sq.Expr("rcu.resource_type_id = t.id"),
			sq.Eq{
				"t.active": false,
			},
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (f *resourceConfigFactory) CleanConfigUsesForInactiveResources() error {
	tx, err := f.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = psql.Delete("resource_config_uses rcu USING resources r").
		Where(sq.And{
			sq.Expr("rcu.resource_id = r.id"),
			sq.Eq{
				"r.active": false,
			},
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (f *resourceConfigFactory) CleanUselessConfigs() error {
	tx, err := f.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

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
		RunWith(tx).Exec()
	if err != nil {
		return err
	}

	return tx.Commit()
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
