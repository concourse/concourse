package postgresrunner

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db"
	"github.com/onsi/ginkgo/v2"
)

func validateJoinLimit(query string) error {
	// Each subquery can have up to <limit> explicit JOINs
	limit := 8
	individualQueries := ExtractQueries(query)
	for _, q := range individualQueries {
		numJoins := strings.Count(strings.ToUpper(q), "JOIN")
		if numJoins > limit {
			errMsg := "the following query has too many JOINs\n\n" + q
			if len(individualQueries) > 1 {
				errMsg += "\n\nit is part of this full query:\n\n" + query
			}
			errMsg += fmt.Sprintf("\n\njoin_collapse_limit defaults to %d JOINs, while this query has %d. exceeding the limit can result in really slow queries", limit, numJoins)

			return fmt.Errorf(errMsg)
		}
	}
	return nil
}

// ExtractQueries is an helper to extract a list of queries from a top-level
// query. In particular, it separates any CTE's, UNIONs, and subqueries from
// the main query, since they are each evaluated separately when calculating
// the number of joins.
func ExtractQueries(query string) []string {
	var queries []string

	var recurse func(int) int
	recurse = func(i int) int {
		start := i
		curQuery := ""
		for i < len(query) {
			if query[i] == ')' {
				break
			}
			if query[i] == '(' {
				curQuery += query[start:i] + "(...)"
				start = recurse(i+1) + 1
				i = start
			}
			i += 1
		}
		if start < len(query) {
			curQuery += query[start:i]
		}
		j := 0
		for j < len(curQuery) {
			unionIndex := strings.Index(strings.ToUpper(curQuery[j:]), "UNION")
			if unionIndex < 0 {
				unionIndex = len(curQuery)
			} else {
				unionIndex += j
			}
			queries = append(queries, strings.TrimSpace(curQuery[j:unionIndex]))
			j = unionIndex + 5
		}
		return i
	}

	i := 0
	for i < len(query) {
		i = recurse(i)
	}
	return queries
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
