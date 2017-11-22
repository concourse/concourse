package db

import (
	"fmt"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
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
	FindResourceConfig(
		logger lager.Logger,
		resourceType string,
		source atc.Source,
		resourceTypes creds.VersionedResourceTypes,
	) (*UsedResourceConfig, bool, error)

	CleanUnreferencedConfigs() error
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
	resourceTypes creds.VersionedResourceTypes,
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
	defer Rollback(tx)

	usedResourceConfig, found, err := resourceConfig.Find(tx)
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

// constructResourceConfig cannot be called for constructing a resource type's
// resource config while also containing the same resource type in the list of
// resource types, because that results in a circular dependency.
func constructResourceConfig(
	resourceTypeName string,
	source atc.Source,
	resourceTypes creds.VersionedResourceTypes,
) (ResourceConfig, error) {
	resourceConfig := ResourceConfig{
		Source: source,
	}

	customType, found := resourceTypes.Lookup(resourceTypeName)
	if found {
		source, err := customType.Source.Evaluate()
		if err != nil {
			return ResourceConfig{}, err
		}

		customTypeResourceConfig, err := constructResourceConfig(
			customType.Type,
			source,
			resourceTypes.Without(customType.Name),
		)
		if err != nil {
			return ResourceConfig{}, err
		}

		// TODO: https://github.com/concourse/concourse/issues/1838
		// if customType.Version == nil {
		// 	return ResourceConfig{}, ErrCustomResourceTypeVersionNotFound{Name: customType.Name}
		// }

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

func (f *resourceConfigFactory) CleanUnreferencedConfigs() error {
	usedByResourceConfigCheckSessionIds, _, err := sq.
		Select("resource_config_id").
		From("resource_config_check_sessions").
		ToSql()
	if err != nil {
		return err
	}

	usedByResourceCachesIds, _, err := sq.
		Select("resource_config_id").
		From("resource_caches").
		ToSql()
	if err != nil {
		return err
	}

	_, err = psql.Delete("resource_configs").
		Where("id NOT IN (" + usedByResourceConfigCheckSessionIds + " UNION " + usedByResourceCachesIds + ")").
		PlaceholderFormat(sq.Dollar).
		RunWith(f.conn).Exec()
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqFKeyViolationErrCode {
			// this can happen if a use or resource cache is created referencing the
			// config; as the subqueries above are not atomic
			return nil
		}

		return err
	}

	return nil
}
