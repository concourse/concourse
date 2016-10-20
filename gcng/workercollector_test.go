package gcng_test

import (
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/gcng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkerCollector", func() {
	var (
		workerCollector gcng.WorkerCollector

		dbConn        dbng.Conn
		teamFactory   dbng.TeamFactory
		workerFactory dbng.WorkerFactory
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = dbng.Wrap(postgresRunner.Open())
		teamFactory = dbng.NewTeamFactory(dbConn)
		workerFactory = dbng.NewWorkerFactory(dbConn)

		logger := lagertest.NewTestLogger("volume-collector")
		workerCollector = gcng.NewWorkerCollector(
			logger,
			workerFactory,
		)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Run", func() {
		BeforeEach(func() {
			_, err := teamFactory.CreateTeam("some-team")
			Expect(err).ToNot(HaveOccurred())

			_, err = workerFactory.SaveWorker(atc.Worker{
				Name:       "some-stallable-worker",
				GardenAddr: "1.2.3.4:7777",
			}, -1*time.Minute)
			Expect(err).ToNot(HaveOccurred())

			_, err = workerFactory.SaveWorker(atc.Worker{
				Name:       "some-immortal-worker",
				GardenAddr: "1.2.3.4:8888",
			}, 0)
			Expect(err).ToNot(HaveOccurred())

			worker1, found1, err := workerFactory.GetWorker("some-stallable-worker")
			Expect(err).NotTo(HaveOccurred())
			Expect(found1).To(BeTrue())
			Expect(string(worker1.State)).To(Equal("running"))
			worker2, found2, err := workerFactory.GetWorker("some-immortal-worker")
			Expect(err).NotTo(HaveOccurred())
			Expect(found2).To(BeTrue())
			Expect(string(worker2.State)).To(Equal("running"))
		})

		It("marks expired workers as `stalled`", func() {
			err := workerCollector.Run()
			Expect(err).NotTo(HaveOccurred())

			By("not removing the worker from the DB")
			worker1, found1, err := workerFactory.GetWorker("some-stallable-worker")
			Expect(err).NotTo(HaveOccurred())
			Expect(found1).To(BeTrue())

			By("changing the state in the DB")
			Expect(string(worker1.State)).To(Equal("stalled"))

			By("leaving workers that haven't expired untouched")
			worker2, found2, err := workerFactory.GetWorker("some-immortal-worker")
			Expect(err).NotTo(HaveOccurred())
			Expect(found2).To(BeTrue())
			Expect(string(worker2.State)).To(Equal("running"))
		})
	})
})
