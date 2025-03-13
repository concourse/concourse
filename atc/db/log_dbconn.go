package db

import (
	"context"
	"database/sql"
	"strings"

	"code.cloudfoundry.org/lager/v3"
	"github.com/Masterminds/squirrel"
)

// Log returns a DbConn wrapper that automatically logs all database operations.
// This wrapper intercepts all database calls and logs their SQL queries before
// passing the operations to the underlying connection. This is useful for
// debugging database interactions and query performance.
//
// The wrapper ensures that all database operations - both regular and context-aware
// variants - are consistently logged using the provided logger. This includes
// transactions, queries, and statement preparations.
func Log(logger lager.Logger, conn DbConn) DbConn {
	return &logDbConn{
		DbConn: conn,
		logger: logger,
	}
}

type logDbConn struct {
	DbConn

	logger lager.Logger
}

func (c *logDbConn) Begin() (Tx, error) {
	tx, err := c.DbConn.Begin()
	if err != nil {
		return nil, err
	}

	return &logDbTx{Tx: tx, logger: c.logger}, nil
}

func (c *logDbConn) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := c.DbConn.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &logDbTx{Tx: tx, logger: c.logger}, nil
}

func (c *logDbConn) Query(query string, args ...any) (*sql.Rows, error) {
	c.logger.Debug("query", lager.Data{"query": strip(query)})
	return c.DbConn.Query(query, args...)
}

func (c *logDbConn) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	c.logger.Debug("query-context", lager.Data{"query": strip(query)})
	return c.DbConn.QueryContext(ctx, query, args...)
}

func (c *logDbConn) QueryRow(query string, args ...any) squirrel.RowScanner {
	c.logger.Debug("query-row", lager.Data{"query": strip(query)})
	return c.DbConn.QueryRow(query, args...)
}

func (c *logDbConn) QueryRowContext(ctx context.Context, query string, args ...any) squirrel.RowScanner {
	c.logger.Debug("query-row-context", lager.Data{"query": strip(query)})
	return c.DbConn.QueryRowContext(ctx, query, args...)
}

func (c *logDbConn) Exec(query string, args ...any) (sql.Result, error) {
	c.logger.Debug("exec", lager.Data{"query": strip(query)})
	return c.DbConn.Exec(query, args...)
}

func (c *logDbConn) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	c.logger.Debug("exec-context", lager.Data{"query": strip(query)})
	return c.DbConn.ExecContext(ctx, query, args...)
}

func (c *logDbConn) Prepare(query string) (*sql.Stmt, error) {
	c.logger.Debug("prepare", lager.Data{"query": strip(query)})
	return c.DbConn.Prepare(query)
}

func (c *logDbConn) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	c.logger.Debug("prepare-context", lager.Data{"query": strip(query)})
	return c.DbConn.PrepareContext(ctx, query)
}

// strip formats a SQL query for logging by removing excess whitespace,
// normalizing line breaks, and truncating very long queries.
// This makes the log output more readable and consistent while
// preserving the essential structure of the query.
func strip(query string) string {
	// Replace all newlines and tabs with spaces
	spaced := strings.ReplaceAll(strings.ReplaceAll(query, "\n", " "), "\t", " ")

	// Split by whitespace and rejoin with a single space to normalize
	normalized := strings.Join(strings.Fields(spaced), " ")

	// Truncate very long queries for readability in logs
	maxLen := 1000
	if len(normalized) > maxLen {
		return normalized[:maxLen] + "... [truncated]"
	}

	return normalized
}

type logDbTx struct {
	Tx

	logger lager.Logger
}

func (t *logDbTx) Query(query string, args ...any) (*sql.Rows, error) {
	t.logger.Debug("tx-query", lager.Data{"query": strip(query)})
	return t.Tx.Query(query, args...)
}

func (t *logDbTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	t.logger.Debug("tx-query-context", lager.Data{"query": strip(query)})
	return t.Tx.QueryContext(ctx, query, args...)
}

func (t *logDbTx) QueryRow(query string, args ...any) squirrel.RowScanner {
	t.logger.Debug("tx-query-row", lager.Data{"query": strip(query)})
	return t.Tx.QueryRow(query, args...)
}

func (t *logDbTx) QueryRowContext(ctx context.Context, query string, args ...any) squirrel.RowScanner {
	t.logger.Debug("tx-query-row-context", lager.Data{"query": strip(query)})
	return t.Tx.QueryRowContext(ctx, query, args...)
}

func (t *logDbTx) Exec(query string, args ...any) (sql.Result, error) {
	t.logger.Debug("tx-exec", lager.Data{"query": strip(query)})
	return t.Tx.Exec(query, args...)
}

func (t *logDbTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	t.logger.Debug("tx-exec-context", lager.Data{"query": strip(query)})
	return t.Tx.ExecContext(ctx, query, args...)
}

func (t *logDbTx) Prepare(query string) (*sql.Stmt, error) {
	t.logger.Debug("tx-prepare", lager.Data{"query": strip(query)})
	return t.Tx.Prepare(query)
}

func (t *logDbTx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	t.logger.Debug("tx-prepare-context", lager.Data{"query": strip(query)})
	return t.Tx.PrepareContext(ctx, query)
}
