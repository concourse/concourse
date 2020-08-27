package db

import (
	"code.cloudfoundry.org/lager"
	"context"
	"database/sql"
	sq "github.com/Masterminds/squirrel"
)

type resourceVersionReaper struct {
	conn                   Conn
	logger                 lager.Logger
	maxVersionsPerResource int
}

func NewResourceVersionReaper(logger lager.Logger, conn Conn, maxVersionsPerResource int) *resourceVersionReaper {
	return &resourceVersionReaper{
		conn:                   conn,
		logger:                 logger,
		maxVersionsPerResource: maxVersionsPerResource,
	}
}

func (r *resourceVersionReaper) ReapVersions(ctx context.Context) error {
	rcsIds, err := r.getResourceConfigScopesToReap(ctx)
	if err != nil {
		return err
	}

	for _, rcsId := range rcsIds {
		checkOrder, err := r.getLastCheckOrderToRetain(ctx, rcsId)
		if err != nil {
			return nil
		}

		if checkOrder == 0 {
			continue
		}

		n, err := r.deleteVersions(ctx, rcsId, checkOrder)
		if err != nil {
			r.logger.Error("failed-to-delete-verions", err, lager.Data{"resource-config-scope-id": rcsId})
			continue
		}
		r.logger.Info("deleted-versions", lager.Data{"resource-config-scope-id": rcsId, "deleted-versions": n})
	}

	return nil
}

// getResourceConfigScopesToReap return a list of resource_config_scope_id who
// has more than maxVersionsPerResource versions.
func (r *resourceVersionReaper) getResourceConfigScopesToReap(ctx context.Context) ([]int, error) {
	rows, err := psql.Select("count(*) a", "resource_config_scope_id").
		From("resource_config_versions").
		Where(sq.NotEq{
			"check_order": 0,
		}).
		Having(sq.Gt{
			"count(*)": r.maxVersionsPerResource,
		}).
		OrderBy("a DESC").
		RunWith(r.conn).
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}

	rcsIds := []int{}
	for rows.Next() {
		var count, rcsId int
		err = rows.Scan(&count, &rcsId)
		if err != nil {
			return nil, err
		}

		rcsIds = append(rcsIds, rcsId)
	}

	return rcsIds, nil
}

// getLastCheckOrderToRetain return a check_order of specified resource_config_scope_id.
// Versions whose check_order are smaller than returned check_order can be deleted.
func (r *resourceVersionReaper) getLastCheckOrderToRetain(ctx context.Context, rcsId int) (int, error) {
	var checkOrder int
	err := r.conn.QueryRowContext(ctx, `
		SELECT check_order
		FROM (
			SELECT check_order
			FROM resource_config_versions
			WHERE resource_config_scope_id = $1 and check_order != 0
            ORDER BY check_order DESC
            LIMIT $2
        ) AS rcv
		ORDER BY check_order
		LIMIT 1`, rcsId, r.maxVersionsPerResource).Scan(&checkOrder)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return checkOrder, nil
}

// deleteVersions deletes unpinned versions whose check_order are smaller than lastCheckOrderToRetain.
func (r *resourceVersionReaper) deleteVersions(ctx context.Context, rcsId int, lastCheckOrderToRetain int) (int64, error) {
	result, err := r.conn.ExecContext(ctx, `
		DELETE FROM resource_config_versions
        WHERE resource_config_scope_id = $1 and check_order < $2 and check_order != 0 and version not in (
			SELECT version
			FROM resource_pins
			WHERE resource_id in (
				SELECT resource_id
				FROM resource_config_scopes
				WHERE id = $1
			)
        )`, rcsId, lastCheckOrderToRetain)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
