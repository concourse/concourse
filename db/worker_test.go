package db_test

import (
	"time"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/types"
)

var _ = Describe("Worker", func() {
	var (
		atcWorker atc.Worker
		worker    Worker
	)

	BeforeEach(func() {
		atcWorker = atc.Worker{
			GardenAddr:       "some-garden-addr",
			BaggageclaimURL:  "some-bc-url",
			HTTPProxyURL:     "some-http-proxy-url",
			HTTPSProxyURL:    "some-https-proxy-url",
			NoProxy:          "some-no-proxy",
			ActiveContainers: 140,
			ResourceTypes: []atc.WorkerResourceType{
				{
					Type:    "some-resource-type",
					Image:   "some-image",
					Version: "some-version",
				},
				{
					Type:    "other-resource-type",
					Image:   "other-image",
					Version: "other-version",
				},
			},
			Platform:  "some-platform",
			Tags:      atc.Tags{"some", "tags"},
			Name:      "some-name",
			StartTime: 55912945,
		}
	})

	Describe("Land", func() {

		BeforeEach(func() {
			var err error
			worker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the worker is present", func() {

			It("marks the worker as `landing`", func() {
				err := worker.Land()
				Expect(err).NotTo(HaveOccurred())

				_, err = worker.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(worker.Name()).To(Equal(atcWorker.Name))
				Expect(worker.State()).To(Equal(WorkerStateLanding))
			})

			Context("when worker is already landed", func() {
				BeforeEach(func() {
					err := worker.Land()
					Expect(err).NotTo(HaveOccurred())
					_, err = workerLifecycle.LandFinishedLandingWorkers()
					Expect(err).NotTo(HaveOccurred())
				})

				It("keeps worker state as landed", func() {
					err := worker.Land()
					Expect(err).NotTo(HaveOccurred())
					_, err = worker.Reload()
					Expect(err).NotTo(HaveOccurred())

					Expect(worker.Name()).To(Equal(atcWorker.Name))
					Expect(worker.State()).To(Equal(WorkerStateLanded))
				})
			})
		})

		Context("when the worker is not present", func() {
			It("returns an error", func() {
				err := worker.Delete()
				Expect(err).NotTo(HaveOccurred())

				err = worker.Land()
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(ErrWorkerNotPresent))
			})
		})
	})

	Describe("Retire", func() {
		BeforeEach(func() {
			var err error
			worker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the worker is present", func() {
			It("marks the worker as `retiring`", func() {
				err := worker.Retire()
				Expect(err).NotTo(HaveOccurred())

				_, err = worker.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(worker.Name()).To(Equal(atcWorker.Name))
				Expect(worker.State()).To(Equal(WorkerStateRetiring))
			})
		})

		Context("when the worker is not present", func() {
			BeforeEach(func() {
				err := worker.Delete()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				err := worker.Retire()
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(ErrWorkerNotPresent))
			})
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			var err error
			worker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the record for the worker", func() {
			err := worker.Delete()
			Expect(err).NotTo(HaveOccurred())

			_, found, err := workerFactory.GetWorker(atcWorker.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("Prune", func() {
		Context("when worker exists", func() {
			DescribeTable("worker in state",
				func(workerState string, errMatch GomegaMatcher) {
					worker, err := workerFactory.SaveWorker(atc.Worker{
						Name:       "worker-to-prune",
						GardenAddr: "1.2.3.4",
						State:      workerState,
					}, 5*time.Minute)
					Expect(err).NotTo(HaveOccurred())

					err = worker.Prune()
					Expect(err).To(errMatch)
				},

				Entry("running", "running", Equal(ErrCannotPruneRunningWorker)),
				Entry("landing", "landing", BeNil()),
				Entry("retiring", "retiring", BeNil()),
			)

			Context("when worker is stalled", func() {
				var pruneErr error
				BeforeEach(func() {
					worker, err := workerFactory.SaveWorker(atc.Worker{
						Name:       "worker-to-prune",
						GardenAddr: "1.2.3.4",
						State:      "running",
					}, -5*time.Minute)
					Expect(err).NotTo(HaveOccurred())

					_, err = workerLifecycle.StallUnresponsiveWorkers()
					Expect(err).NotTo(HaveOccurred())
					pruneErr = worker.Prune()
				})

				It("does not return error", func() {
					Expect(pruneErr).NotTo(HaveOccurred())
				})
			})
		})

		Context("when worker does not exist", func() {
			BeforeEach(func() {
				var err error
				worker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
				err = worker.Delete()
				Expect(err).NotTo(HaveOccurred())
			})

			It("raises ErrWorkerNotPresent", func() {
				err := worker.Prune()
				Expect(err).To(Equal(ErrWorkerNotPresent))
			})
		})
	})

})
