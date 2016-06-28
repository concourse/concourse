package db_test

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Explain", func() {
	var (
		logger         *lagertest.TestLogger
		fakeClock      *fakeclock.FakeClock
		underlyingConn *dbfakes.FakeConn
		explainConn    db.Conn
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("explain")
		fakeClock = fakeclock.NewFakeClock(time.Now())
		underlyingConn = new(dbfakes.FakeConn)
		explainConn = db.Explain(logger, underlyingConn, fakeClock, 100*time.Millisecond)
	})

	AfterEach(func() {
		err := explainConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	It("passes through calls to the underlying connection", func() {
		explainConn.Ping()

		Expect(underlyingConn.PingCallCount()).To(Equal(1))
	})

	It("returns the return values from the underlying connection", func() {
		underlyingConn.PingReturns(errors.New("disaster"))

		err := explainConn.Ping()
		Expect(err).To(MatchError("disaster"))
	})

	Context("when the query takes less time than the timeout", func() {
		var realConn *sql.DB

		BeforeEach(func() {
			postgresRunner.Truncate()

			realConn = postgresRunner.Open()
			realConn.SetMaxOpenConns(2) // +1 for concurrent EXPLAIN
			underlyingConn.QueryStub = func(query string, args ...interface{}) (*sql.Rows, error) {
				return realConn.Query(query, args...)
			}
		})

		AfterEach(func() {
			err := realConn.Close()
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not EXPLAIN the query", func() {
			rows, err := explainConn.Query("SELECT $1::int", 1)
			Expect(err).NotTo(HaveOccurred())

			err = rows.Close()
			Expect(err).NotTo(HaveOccurred())

			Expect(underlyingConn.QueryCallCount()).To(Equal(1))

			query, args := underlyingConn.QueryArgsForCall(0)
			Expect(query).To(Equal("SELECT $1::int"))
			Expect(args).To(Equal(varargs(1)))
		})
	})

	Context("when the query takes more time than the timeout", func() {
		var realConn *sql.DB

		BeforeEach(func() {
			postgresRunner.Truncate()

			realConn = postgresRunner.Open()
			realConn.SetMaxOpenConns(2) // +1 for concurrent EXPLAIN
			underlyingConn.QueryStub = func(query string, args ...interface{}) (*sql.Rows, error) {
				if !strings.HasPrefix(query, "EXPLAIN") {
					fakeClock.Increment(120 * time.Millisecond)
				}

				return realConn.Query(query, args...)
			}

			underlyingConn.QueryRowStub = func(query string, args ...interface{}) *sql.Row {
				if !strings.HasPrefix(query, "EXPLAIN") {
					fakeClock.Increment(120 * time.Millisecond)
				}

				return realConn.QueryRow(query, args...)
			}

			underlyingConn.ExecStub = func(query string, args ...interface{}) (sql.Result, error) {
				if !strings.HasPrefix(query, "EXPLAIN") {
					fakeClock.Increment(120 * time.Millisecond)
				}

				return realConn.Exec(query, args...)
			}
		})

		AfterEach(func() {
			err := realConn.Close()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the explain fails", func() {
			BeforeEach(func() {
				underlyingConn.QueryStub = func(query string, args ...interface{}) (*sql.Rows, error) {
					if strings.HasPrefix(query, "EXPLAIN") {
						return nil, errors.New("disaster!")
					} else {
						fakeClock.Increment(120 * time.Millisecond)
						return realConn.Query(query, args...)
					}
				}
			})

			It("logs an error but does not affect the outcome of the original query", func() {
				rows, err := explainConn.Query("SELECT $1::int", 1)
				Expect(err).NotTo(HaveOccurred())

				err = rows.Close()
				Expect(err).NotTo(HaveOccurred())

				Expect(logger).To(gbytes.Say("disaster!"))
			})
		})

		Describe("Query()", func() {
			It("EXPLAINs the query", func() {
				rows, err := explainConn.Query("SELECT $1::int", 1)
				Expect(err).NotTo(HaveOccurred())

				err = rows.Close()
				Expect(err).NotTo(HaveOccurred())

				Expect(underlyingConn.QueryCallCount()).To(Equal(2))

				query, args := underlyingConn.QueryArgsForCall(0)
				Expect(query).To(Equal("SELECT $1::int"))
				Expect(args).To(Equal(varargs(1)))

				query, args = underlyingConn.QueryArgsForCall(1)
				Expect(query).To(Equal("EXPLAIN SELECT $1::int"))
				Expect(args).To(Equal(varargs(1)))
			})

			It("logs the output of the explain", func() {
				rows, err := explainConn.Query("SELECT $1::int", 1)
				Expect(err).NotTo(HaveOccurred())

				err = rows.Close()
				Expect(err).NotTo(HaveOccurred())

				Expect(logger).To(gbytes.Say("Result"))
				Expect(logger).To(gbytes.Say("cost="))
				Expect(logger).To(gbytes.Say("SELECT"))
			})
		})

		Describe("QueryRow()", func() {
			It("EXPLAINs the query", func() {
				var i int
				err := explainConn.QueryRow("SELECT $1::int", 1).Scan(&i)
				Expect(err).NotTo(HaveOccurred())

				Expect(underlyingConn.QueryRowCallCount()).To(Equal(1))
				Expect(underlyingConn.QueryCallCount()).To(Equal(1))

				query, args := underlyingConn.QueryRowArgsForCall(0)
				Expect(query).To(Equal("SELECT $1::int"))
				Expect(args).To(Equal(varargs(1)))

				query, args = underlyingConn.QueryArgsForCall(0)
				Expect(query).To(Equal("EXPLAIN SELECT $1::int"))
				Expect(args).To(Equal(varargs(1)))
			})

			It("logs the output of the explain", func() {
				var i int
				err := explainConn.QueryRow("SELECT $1::int", 1).Scan(&i)
				Expect(err).NotTo(HaveOccurred())

				Expect(logger).To(gbytes.Say("Result"))
				Expect(logger).To(gbytes.Say("cost="))
				Expect(logger).To(gbytes.Say("SELECT"))
			})
		})

		Describe("Exec()", func() {
			It("EXPLAINs the query", func() {
				_, err := explainConn.Exec("SELECT $1::int", 1)
				Expect(err).NotTo(HaveOccurred())

				Expect(underlyingConn.ExecCallCount()).To(Equal(1))
				Expect(underlyingConn.QueryCallCount()).To(Equal(1))

				query, args := underlyingConn.ExecArgsForCall(0)
				Expect(query).To(Equal("SELECT $1::int"))
				Expect(args).To(Equal(varargs(1)))

				query, args = underlyingConn.QueryArgsForCall(0)
				Expect(query).To(Equal("EXPLAIN SELECT $1::int"))
				Expect(args).To(Equal(varargs(1)))
			})

			It("logs the output of the explain", func() {
				_, err := explainConn.Exec("SELECT $1::int", 1)
				Expect(err).NotTo(HaveOccurred())

				Expect(logger).To(gbytes.Say("Result"))
				Expect(logger).To(gbytes.Say("cost="))
				Expect(logger).To(gbytes.Say("SELECT"))
			})
		})
	})
})

func varargs(x ...interface{}) interface{} {
	return x
}
