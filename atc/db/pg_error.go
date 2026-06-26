package db

import (
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

func isForeignKeyOrRestrictViolation(err error) bool {
	pgErr, ok := err.(*pgconn.PgError)
	if !ok {
		return false
	}

	return pgErr.Code == pgerrcode.ForeignKeyViolation || // Returned by Postgresql <= 17
		pgErr.Code == pgerrcode.RestrictViolation // Returned by Postgresql >= 18
}
