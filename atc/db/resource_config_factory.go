package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
)

type ErrCustomResourceTypeVersionNotFound struct {
	Name string
}

func (e ErrCustomResourceTypeVersionNotFound) Error() string {
	return fmt.Sprintf("custom resource type '%s' version not found", e.Name)
}

//counterfeiter:generate . ResourceConfigFactory
type ResourceConfigFactory interface {
	FindOrCreateResourceConfig(
		resourceType string,
		source atc.Source,
		customTypeResourceCache ResourceCache,
	) (ResourceConfig, error)

	FindResourceConfigByID(int) (ResourceConfig, bool, error)

	CleanUnreferencedConfigs(time.Duration) error
}

type resourceConfigFactory struct {
	conn        DbConn
	lockFactory lock.LockFactory
}

func NewResourceConfigFactory(conn DbConn, lockFactory lock.LockFactory) ResourceConfigFactory {
	return &resourceConfigFactory{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

func (f *resourceConfigFactory) FindResourceConfigByID(resourceConfigID int) (ResourceConfig, bool, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, false, err
	}
	defer Rollback(tx)

	resourceConfig, found, err := findResourceConfigByID(tx, resourceConfigID, f.lockFactory, f.conn)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return resourceConfig, true, nil
}

func findOrCreateResourceConfig(
	tx Tx,
	rc *resourceConfig,
	resourceType string,
	source atc.Source,
	customTypeResourceCache ResourceCache,
	updateLastReferenced bool,
) error {

	var (
		parentID         int
		parentColumnName string
		err              error
		found            bool
	)

	if customTypeResourceCache != nil {
		parentColumnName = "resource_cache_id"
		rc.createdByResourceCache = customTypeResourceCache
		parentID = rc.createdByResourceCache.ID()
	} else {
		parentColumnName = "base_resource_type_id"
		rc.createdByBaseResourceType, found, err = BaseResourceType{Name: resourceType}.Find(tx)
		if err != nil {
			return err
		}

		if !found {
			return BaseResourceTypeNotFoundError{Name: resourceType}
		}

		parentID = rc.CreatedByBaseResourceType().ID
	}

	found = true
	if updateLastReferenced {
		// updateLastReferenced is called extremely frequently, which generates a lot of
		// slow queries. However, last_referenced is only used for gc, it doesn't have to
		// be super precise. Based on that, let's query first, and if last_update is within
		// 1 minutes, then skip current update.
		err := psql.Select("id", "last_referenced").
			From("resource_configs").
			Where(sq.Eq{
				parentColumnName: parentID,
				"source_hash":    mapHash(source),
			}).
			RunWith(tx).
			QueryRow().
			Scan(&rc.id, &rc.lastReferenced)
		if err != nil {
			if err == sql.ErrNoRows {
				found = false
			} else {
				return err
			}
		}

		if found && rc.lastReferenced.Add(time.Minute).Before(time.Now()) {
			err := psql.Update("resource_configs").
				Set("last_referenced", sq.Expr("now()")).
				Where(sq.Eq{
					parentColumnName: parentID,
					"source_hash":    mapHash(source),
				}).
				Suffix("RETURNING id, last_referenced").
				RunWith(tx).
				QueryRow().
				Scan(&rc.id, &rc.lastReferenced)
			if err != nil {
				return err
			}
		}
	} else {
		err := psql.Select("id", "last_referenced").
			From("resource_configs").
			Where(sq.Eq{
				parentColumnName: parentID,
				"source_hash":    mapHash(source),
			}).
			Suffix("FOR SHARE").
			RunWith(tx).
			QueryRow().
			Scan(&rc.id, &rc.lastReferenced)
		if err != nil {
			if err == sql.ErrNoRows {
				found = false
			} else {
				return err
			}
		}
	}

	if !found {
		hash := mapHash(source)

		valueMap := map[string]interface{}{
			parentColumnName: parentID,
			"source_hash":    hash,
		}
		if updateLastReferenced {
			valueMap["last_referenced"] = sq.Expr("now()")
		}
		var updateLastReferencedStr string
		if updateLastReferenced {
			updateLastReferencedStr = `, last_referenced = now()`
		}

		err := psql.Insert("resource_configs").
			SetMap(valueMap).
			Suffix(`
				ON CONFLICT (`+parentColumnName+`, source_hash) DO UPDATE SET
					`+parentColumnName+` = ?,
					source_hash = ?`+
				updateLastReferencedStr+`
				RETURNING id, last_referenced
			`, parentID, hash).
			RunWith(tx).
			QueryRow().
			Scan(&rc.id, &rc.lastReferenced)
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *resourceConfigFactory) FindOrCreateResourceConfig(
	resourceType string,
	source atc.Source,
	customTypeResourceCache ResourceCache,
) (ResourceConfig, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer Rollback(tx)

	rc := &resourceConfig{
		lockFactory: f.lockFactory,
		conn:        f.conn,
	}
	err = findOrCreateResourceConfig(tx, rc, resourceType, source, customTypeResourceCache, true)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return rc, nil
}

func (f *resourceConfigFactory) CleanUnreferencedConfigs(gracePeriod time.Duration) error {
	usedByResourceCachesIds, _, err := sq.
		Select("resource_config_id").
		From("resource_caches").
		ToSql()
	if err != nil {
		return err
	}

	usedByResourcesIds, _, err := sq.
		Select("resource_config_id").
		From("resources").
		Where("resource_config_id IS NOT NULL").
		ToSql()
	if err != nil {
		return err
	}

	usedByResourceTypesIds, _, err := sq.
		Select("resource_config_id").
		From("resource_types").
		Where("resource_config_id IS NOT NULL").
		ToSql()
	if err != nil {
		return err
	}

	usedByPrototypesIds, _, err := sq.
		Select("resource_config_id").
		From("prototypes").
		Where("resource_config_id IS NOT NULL").
		ToSql()
	if err != nil {
		return err
	}

	_, err = psql.Delete("resource_configs").
		Where("id NOT IN (" +
			usedByResourceCachesIds + " UNION " +
			usedByResourcesIds + " UNION " +
			usedByResourceTypesIds + " UNION " +
			usedByPrototypesIds + ")").
		Where(sq.Expr(fmt.Sprintf("now() - last_referenced > '%d seconds'::interval", int(gracePeriod.Seconds())))).
		PlaceholderFormat(sq.Dollar).
		RunWith(f.conn).Exec()
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == pgerrcode.ForeignKeyViolation {
			// this can happen if a use or resource cache is created referencing the
			// config; as the subqueries above are not atomic
			return nil
		}

		return err
	}

	return nil
}

func findResourceConfigByID(tx Tx, resourceConfigID int, lockFactory lock.LockFactory, conn DbConn) (ResourceConfig, bool, error) {
	var brtIDString, cacheIDString sql.NullString

	err := psql.Select("base_resource_type_id", "resource_cache_id").
		From("resource_configs").
		Where(sq.Eq{"id": resourceConfigID}).
		RunWith(tx).
		QueryRow().
		Scan(&brtIDString, &cacheIDString)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	rc := &resourceConfig{
		id:          resourceConfigID,
		lockFactory: lockFactory,
		conn:        conn,
	}

	if brtIDString.Valid {
		var brtName string
		var unique bool
		brtID, err := strconv.Atoi(brtIDString.String)
		if err != nil {
			return nil, false, err
		}

		err = psql.Select("name, unique_version_history").
			From("base_resource_types").
			Where(sq.Eq{"id": brtID}).
			RunWith(tx).
			QueryRow().
			Scan(&brtName, &unique)
		if err != nil {
			return nil, false, err
		}

		rc.createdByBaseResourceType = &UsedBaseResourceType{brtID, brtName, unique}

	} else if cacheIDString.Valid {
		cacheID, err := strconv.Atoi(cacheIDString.String)
		if err != nil {
			return nil, false, err
		}

		usedByResourceCache, found, err := findResourceCacheByID(tx, cacheID, lockFactory, conn)
		if err != nil {
			return nil, false, err
		}

		if !found {
			return nil, false, nil
		}

		rc.createdByResourceCache = usedByResourceCache
	}

	return rc, true, nil
}
