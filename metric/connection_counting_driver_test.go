package metric_test

import (
	"database/sql"
	"errors"
	"fmt"

	. "github.com/concourse/atc/metric"
	"github.com/concourse/atc/metric/metricfakes"
	"github.com/nu7hatch/gouuid"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnectionCountingDriver", func() {
	var (
		delegateDriver           *metricfakes.FakeDriver
		connectionCountingDriver Driver
	)

	BeforeEach(func() {
		delegateDriver = new(metricfakes.FakeDriver)

		uid, err := uuid.NewV4()
		Expect(err).NotTo(HaveOccurred())

		fakeDriverName := fmt.Sprintf("fake-driver-%s", uid)
		sql.Register(fakeDriverName, delegateDriver)

		connectionCountingDriverName := fmt.Sprintf("connection-counting-%s", uid)
		SetupConnectionCountingDriver(fakeDriverName, "dummy-data-source", connectionCountingDriverName)

		dbConn, err := sql.Open(connectionCountingDriverName, "dummy-data-source")
		Expect(err).NotTo(HaveOccurred())
		err = dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
		connectionCountingDriver = dbConn.Driver()
	})

	Describe("connection counting", func() {
		var delegateConn *metricfakes.FakeConn

		BeforeEach(func() {
			delegateConn = new(metricfakes.FakeConn)
			delegateDriver.OpenReturns(delegateConn, nil)
		})

		It("works", func() {
			By("calls open on delegate")
			dbConn, err := connectionCountingDriver.Open("dummy-data-source")
			Expect(err).NotTo(HaveOccurred())

			Expect(delegateDriver.OpenCallCount()).To(Equal(1))
			Expect(delegateDriver.OpenArgsForCall(0)).To(Equal("dummy-data-source"))

			err = dbConn.Close()
			Expect(err).NotTo(HaveOccurred())
			Expect(delegateConn.CloseCallCount()).To(Equal(1))

			By("incrementing the global ;) counter when connection succeeds")
			// reset max to latest counter
			DatabaseConnections.Max()
			maxConnBefore := DatabaseConnections.Max()
			dbConn, err = connectionCountingDriver.Open("dummy-data-source")
			Expect(err).NotTo(HaveOccurred())
			Expect(DatabaseConnections.Max()).To(Equal(maxConnBefore + 1))
			err = dbConn.Close()
			Expect(err).NotTo(HaveOccurred())
			Expect(DatabaseConnections.Max()).To(Equal(maxConnBefore))

			By("not decrementing the global counter when closing fails")
			disaster := errors.New("failed")
			delegateConn.CloseReturns(disaster)
			// reset max to latest counter
			DatabaseConnections.Max()
			maxConnBefore = DatabaseConnections.Max()
			dbConn, err = connectionCountingDriver.Open("dummy-data-source")
			Expect(err).NotTo(HaveOccurred())
			Expect(DatabaseConnections.Max()).To(Equal(maxConnBefore + 1))
			err = dbConn.Close()
			Expect(err).To(Equal(disaster))
			Expect(DatabaseConnections.Max()).To(Equal(maxConnBefore + 1))

			By("not incrementing global counter when connection fails")
			delegateDriver.OpenReturns(nil, disaster)
			// reset max to latest counter
			DatabaseConnections.Max()
			maxConnBefore = DatabaseConnections.Max()
			dbConn, err = connectionCountingDriver.Open("dummy-data-source")
			Expect(err).To(Equal(disaster))
			Expect(dbConn).To(BeNil())
			Expect(DatabaseConnections.Max()).To(Equal(maxConnBefore))
		})
	})
})
