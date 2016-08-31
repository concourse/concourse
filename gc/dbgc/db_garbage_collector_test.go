package dbgc_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/gc/dbgc"
	"github.com/concourse/atc/gc/dbgc/dbgcfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DbGarbageCollector", func() {
	var dbGarbageCollector dbgc.DBGarbageCollector
	var fakeDB *dbgcfakes.FakeReaperDB

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("dbgc")
		fakeDB = new(dbgcfakes.FakeReaperDB)
		dbGarbageCollector = dbgc.NewDBGarbageCollector(logger, fakeDB)
	})

	Describe("Run", func() {
		It("reaps expired containers, workers and volumes", func() {
			err := dbGarbageCollector.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeDB.ReapExpiredContainersCallCount()).To(Equal(1))
			Expect(fakeDB.ReapExpiredVolumesCallCount()).To(Equal(1))
			Expect(fakeDB.ReapExpiredWorkersCallCount()).To(Equal(1))
		})
	})
})
