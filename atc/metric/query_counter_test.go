package metric_test

import (
	"errors"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/metric"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Counting Database Queries", func() {
	var (
		underlyingConn *dbfakes.FakeConn
		countingConn   db.Conn
	)

	BeforeEach(func() {
		underlyingConn = new(dbfakes.FakeConn)
		countingConn = metric.CountQueries(underlyingConn)
	})

	AfterEach(func() {
		err := countingConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	It("passes through calls to the underlying connection", func() {
		countingConn.Ping()

		Expect(underlyingConn.PingCallCount()).To(Equal(1))
	})

	It("returns the return values from the underlying connection", func() {
		underlyingConn.PingReturns(errors.New("disaster"))

		err := countingConn.Ping()
		Expect(err).To(MatchError("disaster"))
	})

	Describe("query counting", func() {
		It("increments the global (;_;) counter", func() {
			_, err := countingConn.Query("SELECT $1::int", 1)
			Expect(err).NotTo(HaveOccurred())

			Expect(metric.DatabaseQueries.Delta()).To(Equal(1))

			_, err = countingConn.Exec("SELECT $1::int", 1)
			Expect(err).NotTo(HaveOccurred())

			countingConn.QueryRow("SELECT $1::int", 1)

			Expect(metric.DatabaseQueries.Delta()).To(Equal(2))

			By("working in transactions")
			underlyingTx := &dbfakes.FakeTx{}
			underlyingConn.BeginReturns(underlyingTx, nil)

			tx, err := countingConn.Begin()
			Expect(err).NotTo(HaveOccurred())

			_, err = tx.Query("SELECT $1::int", 1)
			Expect(err).NotTo(HaveOccurred())

			Expect(metric.DatabaseQueries.Delta()).To(Equal(1))
		})
	})
})
