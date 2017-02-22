package dbng_test

import (
	"database/sql"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker Lifecycle", func() {
	var (
		atcWorker atc.Worker
		worker    dbng.Worker
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
			StartTime: 55,
		}
	})

	Describe("StallUnresponsiveWorkers", func() {
		Context("when the worker has heartbeated recently", func() {
			BeforeEach(func() {
				_, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("leaves the worker alone", func() {
				stalledWorkers, err := workerLifecycle.StallUnresponsiveWorkers()
				Expect(err).NotTo(HaveOccurred())
				Expect(stalledWorkers).To(BeEmpty())
			})
		})

		Context("when the worker has not heartbeated recently", func() {
			BeforeEach(func() {
				_, err = workerFactory.SaveWorker(atcWorker, -1*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("marks the worker as `stalled`", func() {
				stalledWorkers, err := workerLifecycle.StallUnresponsiveWorkers()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(stalledWorkers)).To(Equal(1))
				Expect(stalledWorkers[0]).To(Equal("some-name"))
			})
		})
	})

	Describe("DeleteFinishedRetiringWorkers", func() {
		var (
			dbWorker dbng.Worker
			dbBuild  dbng.Build
		)

		JustBeforeEach(func() {
			var err error
			dbWorker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when worker is not retiring", func() {
			JustBeforeEach(func() {
				var err error
				atcWorker.State = string(dbng.WorkerStateRunning)
				dbWorker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not delete worker", func() {
				_, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				deletedWorkers, err := workerLifecycle.DeleteFinishedRetiringWorkers()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(deletedWorkers)).To(Equal(0))

				_, found, err = workerFactory.GetWorker(atcWorker.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("when worker is retiring", func() {
			BeforeEach(func() {
				atcWorker.State = string(dbng.WorkerStateRetiring)
			})

			Context("when the worker does not have any running builds", func() {
				It("deletes worker", func() {
					_, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					deletedWorkers, err := workerLifecycle.DeleteFinishedRetiringWorkers()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(deletedWorkers)).To(Equal(1))
					Expect(deletedWorkers[0]).To(Equal(atcWorker.Name))

					_, found, err = workerFactory.GetWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeFalse())
				})
			})

			DescribeTable("deleting workers with builds that are",
				func(s dbng.BuildStatus, expectedExistence bool) {
					dbBuild, err := defaultTeam.CreateOneOffBuild()
					Expect(err).NotTo(HaveOccurred())

					err = dbBuild.SaveStatus(s)
					Expect(err).NotTo(HaveOccurred())

					_, err = defaultTeam.CreateBuildContainer(dbWorker.Name(), dbBuild.ID(), atc.PlanID(4), dbng.ContainerMetadata{})
					Expect(err).NotTo(HaveOccurred())

					_, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					_, err = workerLifecycle.DeleteFinishedRetiringWorkers()
					Expect(err).NotTo(HaveOccurred())

					_, found, err = workerFactory.GetWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(Equal(expectedExistence))
				},
				Entry("pending", dbng.BuildStatusPending, true),
				Entry("started", dbng.BuildStatusStarted, true),
				Entry("aborted", dbng.BuildStatusAborted, false),
				Entry("succeeded", dbng.BuildStatusSucceeded, false),
				Entry("failed", dbng.BuildStatusFailed, false),
				Entry("errored", dbng.BuildStatusErrored, false),
			)

			ItRetiresWorkerWithState := func(s dbng.BuildStatus, expectedExistence bool) {
				err := dbBuild.SaveStatus(s)
				Expect(err).NotTo(HaveOccurred())

				_, err = defaultTeam.CreateBuildContainer(dbWorker.Name(), dbBuild.ID(), atc.PlanID(4), dbng.ContainerMetadata{})
				Expect(err).NotTo(HaveOccurred())

				_, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				_, err = workerLifecycle.DeleteFinishedRetiringWorkers()
				Expect(err).NotTo(HaveOccurred())

				_, found, err = workerFactory.GetWorker(atcWorker.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(Equal(expectedExistence))
			}

			Context("when worker has build with uninterruptible job", func() {
				BeforeEach(func() {
					pipeline, created, err := defaultTeam.SavePipeline("some-pipeline", atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name:          "some-job",
								Interruptible: false,
							},
						},
					}, dbng.ConfigVersion(0), dbng.PipelineUnpaused)
					Expect(err).NotTo(HaveOccurred())
					Expect(created).To(BeTrue())

					dbBuild, err = pipeline.CreateJobBuild("some-job")
					Expect(err).NotTo(HaveOccurred())
				})

				DescribeTable("with builds that are",
					ItRetiresWorkerWithState,
					Entry("pending", dbng.BuildStatusPending, true),
					Entry("started", dbng.BuildStatusStarted, true),
					Entry("aborted", dbng.BuildStatusAborted, false),
					Entry("succeeded", dbng.BuildStatusSucceeded, false),
					Entry("failed", dbng.BuildStatusFailed, false),
					Entry("errored", dbng.BuildStatusErrored, false),
				)
			})

			Context("when worker has build with interruptible job", func() {
				BeforeEach(func() {
					pipeline, created, err := defaultTeam.SavePipeline("some-pipeline", atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name:          "some-job",
								Interruptible: true,
							},
						},
					}, dbng.ConfigVersion(0), dbng.PipelineUnpaused)
					Expect(err).NotTo(HaveOccurred())
					Expect(created).To(BeTrue())

					dbBuild, err = pipeline.CreateJobBuild("some-job")
					Expect(err).NotTo(HaveOccurred())
				})

				DescribeTable("with builds that are",
					ItRetiresWorkerWithState,
					Entry("pending", dbng.BuildStatusPending, false),
					Entry("started", dbng.BuildStatusStarted, false),
					Entry("aborted", dbng.BuildStatusAborted, false),
					Entry("succeeded", dbng.BuildStatusSucceeded, false),
					Entry("failed", dbng.BuildStatusFailed, false),
					Entry("errored", dbng.BuildStatusErrored, false),
				)
			})

			Context("when worker has one-off build", func() {
				BeforeEach(func() {
					var err error
					dbBuild, err = defaultTeam.CreateOneOffBuild()
					Expect(err).NotTo(HaveOccurred())
				})

				DescribeTable("with builds that are",
					ItRetiresWorkerWithState,
					Entry("pending", dbng.BuildStatusPending, true),
					Entry("started", dbng.BuildStatusStarted, true),
					Entry("aborted", dbng.BuildStatusAborted, false),
					Entry("succeeded", dbng.BuildStatusSucceeded, false),
					Entry("failed", dbng.BuildStatusFailed, false),
					Entry("errored", dbng.BuildStatusErrored, false),
				)
			})
		})
	})

	Describe("LandFinishedLandingWorkers", func() {
		var (
			dbWorker dbng.Worker
			dbBuild  dbng.Build
		)

		JustBeforeEach(func() {
			var err error
			dbWorker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when worker is not landing", func() {
			JustBeforeEach(func() {
				var err error
				atcWorker.State = string(dbng.WorkerStateRunning)
				dbWorker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not land worker", func() {
				_, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				landedWorkers, err := workerLifecycle.LandFinishedLandingWorkers()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(landedWorkers)).To(Equal(0))

				foundWorker, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(foundWorker.State()).To(Equal(dbng.WorkerStateRunning))
			})
		})

		Context("when worker is landing", func() {
			BeforeEach(func() {
				atcWorker.State = string(dbng.WorkerStateLanding)
			})

			Context("when the worker does not have any running builds", func() {
				It("lands worker", func() {
					_, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					landedWorkers, err := workerLifecycle.LandFinishedLandingWorkers()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(landedWorkers)).To(Equal(1))
					Expect(landedWorkers[0]).To(Equal(atcWorker.Name))

					foundWorker, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(foundWorker.State()).To(Equal(dbng.WorkerStateLanded))
				})

				It("clears out the garden/baggageclaim addresses", func() {
					var (
						a1    sql.NullString
						b1    sql.NullString
						a2    sql.NullString
						b2    sql.NullString
						found bool
						err   error
					)

					worker, found, err = workerFactory.GetWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					dbConn.QueryRow("SELECT addr, baggageclaim_url FROM workers WHERE name = '"+atcWorker.Name+"'").Scan(&a1, &b1)

					Expect(a1.Valid).To(BeTrue())
					Expect(b1.Valid).To(BeTrue())

					err = worker.Land()
					Expect(err).NotTo(HaveOccurred())

					dbConn.QueryRow("SELECT addr, baggageclaim_url FROM workers WHERE name = "+atcWorker.Name+"'").Scan(&a2, &b2)

					Expect(a2.Valid).To(BeFalse())
					Expect(b2.Valid).To(BeFalse())
				})
			})

			DescribeTable("land workers with builds that are",
				func(s dbng.BuildStatus, expectedState dbng.WorkerState) {
					dbBuild, err := defaultTeam.CreateOneOffBuild()
					Expect(err).NotTo(HaveOccurred())

					err = dbBuild.SaveStatus(s)
					Expect(err).NotTo(HaveOccurred())

					_, err = defaultTeam.CreateBuildContainer(dbWorker.Name(), dbBuild.ID(), atc.PlanID(4), dbng.ContainerMetadata{})
					Expect(err).NotTo(HaveOccurred())

					_, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					_, err = workerLifecycle.LandFinishedLandingWorkers()
					Expect(err).NotTo(HaveOccurred())

					foundWorker, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(foundWorker.State()).To(Equal(expectedState))
				},
				Entry("pending", dbng.BuildStatusPending, dbng.WorkerStateLanding),
				Entry("started", dbng.BuildStatusStarted, dbng.WorkerStateLanding),
				Entry("aborted", dbng.BuildStatusAborted, dbng.WorkerStateLanded),
				Entry("succeeded", dbng.BuildStatusSucceeded, dbng.WorkerStateLanded),
				Entry("failed", dbng.BuildStatusFailed, dbng.WorkerStateLanded),
				Entry("errored", dbng.BuildStatusErrored, dbng.WorkerStateLanded),
			)

			ItLandsWorkerWithExpectedState := func(s dbng.BuildStatus, expectedState dbng.WorkerState) {
				err := dbBuild.SaveStatus(s)
				Expect(err).NotTo(HaveOccurred())

				_, err = defaultTeam.CreateBuildContainer(dbWorker.Name(), dbBuild.ID(), atc.PlanID(4), dbng.ContainerMetadata{})
				Expect(err).NotTo(HaveOccurred())

				_, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				_, err = workerLifecycle.LandFinishedLandingWorkers()
				Expect(err).NotTo(HaveOccurred())

				foundWorker, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(foundWorker.State()).To(Equal(expectedState))
			}

			Context("when worker has build with uninterruptible job", func() {
				BeforeEach(func() {
					pipeline, created, err := defaultTeam.SavePipeline("some-pipeline", atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name:          "some-job",
								Interruptible: false,
							},
						},
					}, dbng.ConfigVersion(0), dbng.PipelineUnpaused)
					Expect(err).NotTo(HaveOccurred())
					Expect(created).To(BeTrue())

					dbBuild, err = pipeline.CreateJobBuild("some-job")
					Expect(err).NotTo(HaveOccurred())
				})

				DescribeTable("with builds that are",
					ItLandsWorkerWithExpectedState,
					Entry("pending", dbng.BuildStatusPending, dbng.WorkerStateLanding),
					Entry("started", dbng.BuildStatusStarted, dbng.WorkerStateLanding),
					Entry("aborted", dbng.BuildStatusAborted, dbng.WorkerStateLanded),
					Entry("succeeded", dbng.BuildStatusSucceeded, dbng.WorkerStateLanded),
					Entry("failed", dbng.BuildStatusFailed, dbng.WorkerStateLanded),
					Entry("errored", dbng.BuildStatusErrored, dbng.WorkerStateLanded),
				)
			})

			Context("when worker has build with interruptible job", func() {
				BeforeEach(func() {
					pipeline, created, err := defaultTeam.SavePipeline("some-pipeline", atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name:          "some-job",
								Interruptible: true,
							},
						},
					}, dbng.ConfigVersion(0), dbng.PipelineUnpaused)
					Expect(err).NotTo(HaveOccurred())
					Expect(created).To(BeTrue())

					dbBuild, err = pipeline.CreateJobBuild("some-job")
					Expect(err).NotTo(HaveOccurred())
				})

				DescribeTable("with builds that are",
					ItLandsWorkerWithExpectedState,
					Entry("pending", dbng.BuildStatusPending, dbng.WorkerStateLanded),
					Entry("started", dbng.BuildStatusStarted, dbng.WorkerStateLanded),
					Entry("aborted", dbng.BuildStatusAborted, dbng.WorkerStateLanded),
					Entry("succeeded", dbng.BuildStatusSucceeded, dbng.WorkerStateLanded),
					Entry("failed", dbng.BuildStatusFailed, dbng.WorkerStateLanded),
					Entry("errored", dbng.BuildStatusErrored, dbng.WorkerStateLanded),
				)
			})

			Context("when worker has one-off build", func() {
				BeforeEach(func() {
					var err error
					dbBuild, err = defaultTeam.CreateOneOffBuild()
					Expect(err).NotTo(HaveOccurred())
				})

				DescribeTable("with builds that are",
					ItLandsWorkerWithExpectedState,
					Entry("pending", dbng.BuildStatusPending, dbng.WorkerStateLanding),
					Entry("started", dbng.BuildStatusStarted, dbng.WorkerStateLanding),
					Entry("aborted", dbng.BuildStatusAborted, dbng.WorkerStateLanded),
					Entry("succeeded", dbng.BuildStatusSucceeded, dbng.WorkerStateLanded),
					Entry("failed", dbng.BuildStatusFailed, dbng.WorkerStateLanded),
					Entry("errored", dbng.BuildStatusErrored, dbng.WorkerStateLanded),
				)
			})
		})
	})

})
