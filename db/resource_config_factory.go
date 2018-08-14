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
	) (ResourceConfig, bool, error)

	FindOrCreateResourceConfig(
		logger lager.Logger,
		resourceType string,
		source atc.Source,
		resourceTypes creds.VersionedResourceTypes,
	) (ResourceConfig, error)

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
) (ResourceConfig, bool, error) {
	resourceConfigDescriptor, err := constructResourceConfigDescriptor(resourceType, source, resourceTypes)
	if err != nil {
		return nil, false, err
	}

	var resourceConfig ResourceConfig

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, false, err
	}
	defer Rollback(tx)

	resourceConfig, found, err := resourceConfigDescriptor.find(tx, f.lockFactory, f.conn)
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

	return resourceConfig, true, nil
}

func (f *resourceConfigFactory) FindOrCreateResourceConfig(
	logger lager.Logger,
	resourceType string,
	source atc.Source,
	resourceTypes creds.VersionedResourceTypes,
) (ResourceConfig, error) {
	resourceConfigDescriptor, err := constructResourceConfigDescriptor(resourceType, source, resourceTypes)
	if err != nil {
		return nil, err
	}

	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer Rollback(tx)

	resourceConfig, err := resourceConfigDescriptor.findOrCreate(logger, tx, f.lockFactory, f.conn)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return resourceConfig, nil
}

// constructResourceConfig cannot be called for constructing a resource type's
// resource config while also containing the same resource type in the list of
// resource types, because that results in a circular dependency.
func constructResourceConfigDescriptor(
	resourceTypeName string,
	source atc.Source,
	resourceTypes creds.VersionedResourceTypes,
) (ResourceConfigDescriptor, error) {
	resourceConfigDescriptor := ResourceConfigDescriptor{
		Source: source,
	}

	customType, found := resourceTypes.Lookup(resourceTypeName)
	if found {
		source, err := customType.Source.Evaluate()
		if err != nil {
			return ResourceConfigDescriptor{}, err
		}

		customTypeResourceConfig, err := constructResourceConfigDescriptor(
			customType.Type,
			source,
			resourceTypes.Without(customType.Name),
		)
		if err != nil {
			return ResourceConfigDescriptor{}, err
		}

		resourceConfigDescriptor.CreatedByResourceCache = &ResourceCacheDescriptor{
			ResourceConfigDescriptor: customTypeResourceConfig,
			Version:                  customType.Version,
		}
	} else {
		resourceConfigDescriptor.CreatedByBaseResourceType = &BaseResourceType{
			Name: resourceTypeName,
		}
	}

	return resourceConfigDescriptor, nil
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
