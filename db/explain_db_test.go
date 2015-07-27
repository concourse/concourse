package db_test

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/fakes"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Explain", func() {
	var (
		logger         *lagertest.TestLogger
		underlyingConn *fakes.FakeConn
		explainConn    db.Conn
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("explain")
		underlyingConn = new(fakes.FakeConn)
		explainConn = db.Explain(logger, underlyingConn, 100*time.Millisecond)
	})

	AfterEach(func() {
		err := explainConn.Close()
		Ω(err).ShouldNot(HaveOccurred())
	})

	It("passes through calls to the underlying connection", func() {
		explainConn.Ping()

		Ω(underlyingConn.PingCallCount()).To(Equal(1))
	})

	It("returns the return values from the underlying connection", func() {
		underlyingConn.PingReturns(errors.New("disaster"))

		err := explainConn.Ping()
		Ω(err).Should(MatchError("disaster"))
	})

	Context("when the query takes less time than the timeout", func() {
		var realConn *sql.DB

		BeforeEach(func() {
			postgresRunner.CreateTestDB()
			realConn = postgresRunner.Open()
			underlyingConn.QueryStub = func(query string, args ...interface{}) (*sql.Rows, error) {
				return realConn.Query(query, args...)
			}
		})

		AfterEach(func() {
			err := realConn.Close()
			Ω(err).ShouldNot(HaveOccurred())

			postgresRunner.DropTestDB()
		})

		It("does not EXPLAIN the query", func() {
			rows, err := explainConn.Query("SELECT $1::int", 1)
			Ω(err).ShouldNot(HaveOccurred())

			err = rows.Close()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(underlyingConn.QueryCallCount()).Should(Equal(1))

			query, args := underlyingConn.QueryArgsForCall(0)
			Ω(query).Should(Equal("SELECT $1::int"))
			Ω(args).Should(Equal(varargs(1)))
		})
	})

	Context("when the query takes more time than the timeout", func() {
		var realConn *sql.DB

		BeforeEach(func() {
			postgresRunner.CreateTestDB()
			realConn = postgresRunner.Open()
			underlyingConn.QueryStub = func(query string, args ...interface{}) (*sql.Rows, error) {
				if !strings.HasPrefix(query, "EXPLAIN") {
					time.Sleep(120 * time.Millisecond)
				}

				return realConn.Query(query, args...)
			}

			underlyingConn.QueryRowStub = func(query string, args ...interface{}) *sql.Row {
				if !strings.HasPrefix(query, "EXPLAIN") {
					time.Sleep(120 * time.Millisecond)
				}

				return realConn.QueryRow(query, args...)
			}

			underlyingConn.ExecStub = func(query string, args ...interface{}) (sql.Result, error) {
				if !strings.HasPrefix(query, "EXPLAIN") {
					time.Sleep(120 * time.Millisecond)
				}

				return realConn.Exec(query, args...)
			}
		})

		AfterEach(func() {
			err := realConn.Close()
			Ω(err).ShouldNot(HaveOccurred())

			postgresRunner.DropTestDB()
		})

		Context("when the explain fails", func() {
			BeforeEach(func() {
				underlyingConn.QueryStub = func(query string, args ...interface{}) (*sql.Rows, error) {
					if strings.HasPrefix(query, "EXPLAIN") {
						return nil, errors.New("disaster!")
					} else {
						time.Sleep(120 * time.Millisecond)
						return realConn.Query(query, args...)
					}
				}
			})

			It("logs an error but does not affect the outcome of the original query", func() {
				rows, err := explainConn.Query("SELECT $1::int", 1)
				Ω(err).ShouldNot(HaveOccurred())

				err = rows.Close()
				Ω(err).ShouldNot(HaveOccurred())

				Expect(logger).To(gbytes.Say("disaster!"))
			})
		})

		Describe("Query()", func() {
			It("EXPLAINs the query", func() {
				rows, err := explainConn.Query("SELECT $1::int", 1)
				Ω(err).ShouldNot(HaveOccurred())

				err = rows.Close()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(underlyingConn.QueryCallCount()).Should(Equal(2))

				query, args := underlyingConn.QueryArgsForCall(0)
				Ω(query).Should(Equal("SELECT $1::int"))
				Ω(args).Should(Equal(varargs(1)))

				query, args = underlyingConn.QueryArgsForCall(1)
				Ω(query).Should(Equal("EXPLAIN SELECT $1::int"))
				Ω(args).Should(Equal(varargs(1)))
			})

			It("logs the output of the explain", func() {
				rows, err := explainConn.Query("SELECT $1::int", 1)
				Ω(err).ShouldNot(HaveOccurred())

				err = rows.Close()
				Ω(err).ShouldNot(HaveOccurred())

				Expect(logger).To(gbytes.Say("Result"))
				Expect(logger).To(gbytes.Say("cost="))
				Expect(logger).To(gbytes.Say("SELECT"))
			})
		})

		Describe("QueryRow()", func() {
			It("EXPLAINs the query", func() {
				var i int
				err := explainConn.QueryRow("SELECT $1::int", 1).Scan(&i)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(underlyingConn.QueryRowCallCount()).Should(Equal(1))
				Ω(underlyingConn.QueryCallCount()).Should(Equal(1))

				query, args := underlyingConn.QueryRowArgsForCall(0)
				Ω(query).Should(Equal("SELECT $1::int"))
				Ω(args).Should(Equal(varargs(1)))

				query, args = underlyingConn.QueryArgsForCall(0)
				Ω(query).Should(Equal("EXPLAIN SELECT $1::int"))
				Ω(args).Should(Equal(varargs(1)))
			})

			It("logs the output of the explain", func() {
				var i int
				err := explainConn.QueryRow("SELECT $1::int", 1).Scan(&i)
				Ω(err).ShouldNot(HaveOccurred())

				Expect(logger).To(gbytes.Say("Result"))
				Expect(logger).To(gbytes.Say("cost="))
				Expect(logger).To(gbytes.Say("SELECT"))
			})
		})

		Describe("Exec()", func() {
			It("EXPLAINs the query", func() {
				_, err := explainConn.Exec("SELECT $1::int", 1)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(underlyingConn.ExecCallCount()).Should(Equal(1))
				Ω(underlyingConn.QueryCallCount()).Should(Equal(1))

				query, args := underlyingConn.ExecArgsForCall(0)
				Ω(query).Should(Equal("SELECT $1::int"))
				Ω(args).Should(Equal(varargs(1)))

				query, args = underlyingConn.QueryArgsForCall(0)
				Ω(query).Should(Equal("EXPLAIN SELECT $1::int"))
				Ω(args).Should(Equal(varargs(1)))
			})

			It("logs the output of the explain", func() {
				_, err := explainConn.Exec("SELECT $1::int", 1)
				Ω(err).ShouldNot(HaveOccurred())

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
