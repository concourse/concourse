package db_test

import (
	"context"
	"database/sql/driver"

	"github.com/concourse/concourse/v8/atc/db"
	"github.com/concourse/concourse/v8/atc/db/dbfakes"
	"github.com/jackc/pgx/v5/pgconn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnectionRetryingDriver", func() {
	Describe("isFailoverError", func() {
		DescribeTable("classifies SQLSTATE codes",
			func(code string, want bool) {
				Expect(db.IsFailoverError(&pgconn.PgError{Code: code})).To(Equal(want))
			},
			Entry("read_only_sql_transaction", "25006", true),
			Entry("admin_shutdown", "57P01", true),
			Entry("crash_shutdown", "57P02", true),
			Entry("serialization_failure", "40001", false),
			Entry("deadlock_detected", "40P01", false),
			Entry("unique_violation", "23505", false),
		)
	})

	Describe("ExecContext", func() {
		DescribeTable("converts failover errors to driver.ErrBadConn",
			func(code string) {
				inner := new(dbfakes.FakeDriverConn)
				inner.ExecContextReturns(nil, &pgconn.PgError{Code: code})
				conn := db.NewRetryingConn(inner)
				_, err := conn.ExecContext(context.Background(), "SELECT 1", nil)
				Expect(err).To(Equal(driver.ErrBadConn))
			},
			Entry("read_only_sql_transaction", "25006"),
			Entry("admin_shutdown", "57P01"),
			Entry("crash_shutdown", "57P02"),
		)

		DescribeTable("passes through non-failover errors",
			func(code string) {
				pgErr := &pgconn.PgError{Code: code}
				inner := new(dbfakes.FakeDriverConn)
				inner.ExecContextReturns(nil, pgErr)
				conn := db.NewRetryingConn(inner)
				_, err := conn.ExecContext(context.Background(), "SELECT 1", nil)
				Expect(err).NotTo(Equal(driver.ErrBadConn))
				Expect(err).To(BeIdenticalTo(pgErr))
			},
			Entry("serialization_failure", "40001"),
			Entry("deadlock_detected", "40P01"),
			Entry("unique_violation", "23505"),
		)

		It("returns driver.ErrSkip when the inner connection lacks ExecerContext", func() {
			type basicConn struct{ driver.Conn }
			conn := db.NewRetryingConn(basicConn{})
			_, err := conn.ExecContext(context.Background(), "SELECT 1", nil)
			Expect(err).To(Equal(driver.ErrSkip))
		})
	})

	Describe("QueryContext", func() {
		It("converts read_only_sql_transaction to driver.ErrBadConn", func() {
			inner := new(dbfakes.FakeDriverConn)
			inner.QueryContextReturns(nil, &pgconn.PgError{Code: "25006"})
			conn := db.NewRetryingConn(inner)
			_, err := conn.QueryContext(context.Background(), "SELECT 1", nil)
			Expect(err).To(Equal(driver.ErrBadConn))
		})
	})

	Describe("CheckNamedValue", func() {
		It("forwards to the inner connection", func() {
			inner := new(dbfakes.FakeDriverConn)
			conn := db.NewRetryingConn(inner)
			nv := &driver.NamedValue{Value: []string{"a", "b"}}
			Expect(conn.CheckNamedValue(nv)).To(Succeed())
			Expect(inner.CheckNamedValueCallCount()).To(Equal(1))
			Expect(inner.CheckNamedValueArgsForCall(0)).To(BeIdenticalTo(nv))
		})

		It("returns driver.ErrSkip when the inner connection lacks NamedValueChecker", func() {
			type basicConn struct{ driver.Conn }
			conn := db.NewRetryingConn(basicConn{})
			Expect(conn.CheckNamedValue(&driver.NamedValue{Value: "x"})).To(Equal(driver.ErrSkip))
		})
	})
})
