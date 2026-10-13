package db

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsFailoverError(t *testing.T) {
	cases := []struct {
		code string
		want bool
	}{
		{"25006", true},  // read_only_sql_transaction
		{"57P01", true},  // admin_shutdown
		{"57P02", true},  // crash_shutdown
		{"40001", false}, // serialization_failure — must not trigger silent retry
		{"40P01", false}, // deadlock_detected — must not trigger silent retry
		{"23505", false}, // unique_violation — normal constraint error
	}
	for _, c := range cases {
		err := &pgconn.PgError{Code: c.code}
		got := isFailoverError(err)
		if got != c.want {
			t.Errorf("isFailoverError({Code: %q}) = %v, want %v", c.code, got, c.want)
		}
	}
}

type mockExecerConn struct {
	execErr  error
	queryErr error
}

func (m *mockExecerConn) Prepare(_ string) (driver.Stmt, error) { return nil, nil }
func (m *mockExecerConn) Close() error                          { return nil }
func (m *mockExecerConn) Begin() (driver.Tx, error) { return nil, nil }
func (m *mockExecerConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	return nil, m.execErr
}
func (m *mockExecerConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	return nil, m.queryErr
}

func TestRetryingConn_ExecContext_ConvertsFailoverErrorsToErrBadConn(t *testing.T) {
	for _, code := range []string{"25006", "57P01", "57P02"} {
		conn := &retryingConn{inner: &mockExecerConn{execErr: &pgconn.PgError{Code: code}}}
		_, err := conn.ExecContext(context.Background(), "SELECT 1", nil)
		if err != driver.ErrBadConn {
			t.Errorf("ExecContext SQLSTATE %s: want driver.ErrBadConn, got %v", code, err)
		}
	}
}

func TestRetryingConn_ExecContext_PassesThroughNonFailoverErrors(t *testing.T) {
	for _, code := range []string{"40001", "40P01", "23505"} {
		pgErr := &pgconn.PgError{Code: code}
		conn := &retryingConn{inner: &mockExecerConn{execErr: pgErr}}
		_, err := conn.ExecContext(context.Background(), "SELECT 1", nil)
		if err == driver.ErrBadConn {
			t.Errorf("ExecContext SQLSTATE %s: must not return driver.ErrBadConn", code)
		}
		if err != pgErr {
			t.Errorf("ExecContext SQLSTATE %s: want original error, got %v", code, err)
		}
	}
}

func TestRetryingConn_QueryContext_ConvertsFailoverErrorsToErrBadConn(t *testing.T) {
	conn := &retryingConn{inner: &mockExecerConn{queryErr: &pgconn.PgError{Code: "25006"}}}
	_, err := conn.QueryContext(context.Background(), "SELECT 1", nil)
	if err != driver.ErrBadConn {
		t.Errorf("QueryContext SQLSTATE 25006: want driver.ErrBadConn, got %v", err)
	}
}

func TestRetryingConn_FallsBackToErrSkipWhenInnerDoesNotImplementExecerContext(t *testing.T) {
	type basicConn struct{ driver.Conn }
	conn := &retryingConn{inner: basicConn{}}
	_, err := conn.ExecContext(context.Background(), "SELECT 1", nil)
	if err != driver.ErrSkip {
		t.Errorf("ExecContext without ExecerContext: want driver.ErrSkip, got %v", err)
	}
}

type mockNamedValueConn struct {
	mockExecerConn
	checkErr error
	called   bool
}

func (m *mockNamedValueConn) CheckNamedValue(nv *driver.NamedValue) error {
	m.called = true
	return m.checkErr
}

func TestRetryingConn_CheckNamedValue_ForwardsToInner(t *testing.T) {
	inner := &mockNamedValueConn{}
	conn := &retryingConn{inner: inner}
	nv := &driver.NamedValue{Value: []string{"a", "b"}}
	err := conn.CheckNamedValue(nv)
	if err != nil {
		t.Errorf("CheckNamedValue: want nil, got %v", err)
	}
	if !inner.called {
		t.Error("CheckNamedValue: inner.CheckNamedValue was not called")
	}
}

func TestRetryingConn_CheckNamedValue_FallsBackToErrSkipWhenInnerLacksChecker(t *testing.T) {
	type basicConn struct{ driver.Conn }
	conn := &retryingConn{inner: basicConn{}}
	err := conn.CheckNamedValue(&driver.NamedValue{Value: "x"})
	if err != driver.ErrSkip {
		t.Errorf("CheckNamedValue without NamedValueChecker: want driver.ErrSkip, got %v", err)
	}
}
