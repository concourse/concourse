package postgresrunner

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db"
	"github.com/onsi/ginkgo"
)

func validateJoinLimit(query string) error {
	// Each subquery, split by a UNION or a UNION ALL, can have up to 8 JOINs
	subQueries := strings.Split(query, "UNION")
	for _, subQuery := range subQueries {
		numJoins := strings.Count(strings.ToUpper(subQuery), "JOIN")
		if numJoins > 8 {
			errMsg := "the following query has too many JOINs\n\n" + subQuery
			if len(subQueries) > 1 {
				errMsg += "\n\nit is part of this full query:\n\n" + query
			}
			errMsg += fmt.Sprintf("\n\njoin_collapse_limit defaults to 8 JOINs, while this query has %d. exceeding the limit can result in really slow queries", numJoins)

			return fmt.Errorf(errMsg)
		}
	}
	return nil
}

type joinLimitValidatorConn struct {
	db.Conn
}

func (c joinLimitValidatorConn) Begin() (db.Tx, error) {
	tx, err := c.Conn.Begin()
	if err != nil {
		return nil, err
	}

	return joinLimitValidatorTx{Tx: tx}, nil
}

func (c joinLimitValidatorConn) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if err := validateJoinLimit(query); err != nil {
		ginkgo.Fail(err.Error())
	}
	return c.Conn.Query(query, args...)
}

func (c joinLimitValidatorConn) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	if err := validateJoinLimit(query); err != nil {
		ginkgo.Fail(err.Error())
	}
	return c.Conn.QueryRow(query, args...)
}

func (c joinLimitValidatorConn) Exec(query string, args ...interface{}) (sql.Result, error) {
	if err := validateJoinLimit(query); err != nil {
		ginkgo.Fail(err.Error())
	}
	return c.Conn.Exec(query, args...)
}

type joinLimitValidatorTx struct {
	db.Tx
}

func (t joinLimitValidatorTx) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if err := validateJoinLimit(query); err != nil {
		ginkgo.Fail(err.Error())
	}
	return t.Tx.Query(query, args...)
}

func (t joinLimitValidatorTx) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	if err := validateJoinLimit(query); err != nil {
		ginkgo.Fail(err.Error())
	}
	return t.Tx.QueryRow(query, args...)
}

func (t joinLimitValidatorTx) Exec(query string, args ...interface{}) (sql.Result, error) {
	if err := validateJoinLimit(query); err != nil {
		ginkgo.Fail(err.Error())
	}
	return t.Tx.Exec(query, args...)
}

func (t joinLimitValidatorTx) QueryRowContext(ctx context.Context, query string, args ...interface{}) squirrel.RowScanner {
	if err := validateJoinLimit(query); err != nil {
		ginkgo.Fail(err.Error())
	}
	return t.Tx.QueryRowContext(ctx, query, args...)
}
