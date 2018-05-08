package db_test

import (
	"database/sql"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker Lifecycle", func() {
	var (
		atcWorker atc.Worker
		worker    db.Worker
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
				_, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).ToNot(HaveOccurred())
			})

			It("leaves the worker alone", func() {
				stalledWorkers, err := workerLifecycle.StallUnresponsiveWorkers()
				Expect(err).ToNot(HaveOccurred())
				Expect(stalledWorkers).To(BeEmpty())
			})
		})

		Context("when the worker has not heartbeated recently", func() {
			BeforeEach(func() {
				_, err := workerFactory.SaveWorker(atcWorker, -1*time.Minute)
				Expect(err).ToNot(HaveOccurred())
			})

			It("marks the worker as `stalled`", func() {
				stalledWorkers, err := workerLifecycle.StallUnresponsiveWorkers()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(stalledWorkers)).To(Equal(1))
				Expect(stalledWorkers[0]).To(Equal("some-name"))
			})
		})
	})

	Describe("DeleteFinishedRetiringWorkers", func() {
		var (
			dbWorker db.Worker
			dbBuild  db.Build
		)

		JustBeforeEach(func() {
			var err error
			dbWorker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when worker is not retiring", func() {
			JustBeforeEach(func() {
				var err error
				atcWorker.State = string(db.WorkerStateRunning)
				dbWorker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not delete worker", func() {
				_, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				deletedWorkers, err := workerLifecycle.DeleteFinishedRetiringWorkers()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(deletedWorkers)).To(Equal(0))

				_, found, err = workerFactory.GetWorker(atcWorker.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("when worker is retiring", func() {
			BeforeEach(func() {
				atcWorker.State = string(db.WorkerStateRetiring)
			})

			Context("when the worker does not have any running builds", func() {
				It("deletes worker", func() {
					_, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					deletedWorkers, err := workerLifecycle.DeleteFinishedRetiringWorkers()
					Expect(err).ToNot(HaveOccurred())
					Expect(len(deletedWorkers)).To(Equal(1))
					Expect(deletedWorkers[0]).To(Equal(atcWorker.Name))

					_, found, err = workerFactory.GetWorker(atcWorker.Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeFalse())
				})
			})

			DescribeTable("deleting workers with builds that are",
				func(s db.BuildStatus, expectedExistence bool) {
					dbBuild, err := defaultTeam.CreateOneOffBuild()
					Expect(err).ToNot(HaveOccurred())

					switch s {
					case db.BuildStatusPending:
					case db.BuildStatusStarted:
						_, err = dbBuild.Start("exec.v2", "{}", atc.Plan{})
						Expect(err).ToNot(HaveOccurred())
					default:
						err = dbBuild.Finish(s)
						Expect(err).ToNot(HaveOccurred())
					}
					_, err = defaultTeam.CreateContainer(dbWorker.Name(), db.NewBuildStepContainerOwner(dbBuild.ID(), atc.PlanID(4)), db.ContainerMetadata{})
					Expect(err).ToNot(HaveOccurred())

					_, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					_, err = workerLifecycle.DeleteFinishedRetiringWorkers()
					Expect(err).ToNot(HaveOccurred())

					_, found, err = workerFactory.GetWorker(atcWorker.Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(Equal(expectedExistence))
				},
				Entry("pending", db.BuildStatusPending, true),
				Entry("started", db.BuildStatusStarted, true),
				Entry("aborted", db.BuildStatusAborted, false),
				Entry("succeeded", db.BuildStatusSucceeded, false),
				Entry("failed", db.BuildStatusFailed, false),
				Entry("errored", db.BuildStatusErrored, false),
			)

			ItRetiresWorkerWithState := func(s db.BuildStatus, expectedExistence bool) {
				switch s {
				case db.BuildStatusPending:
				case db.BuildStatusStarted:
					_, err := dbBuild.Start("exec.v2", "{}", atc.Plan{})
					Expect(err).ToNot(HaveOccurred())
				default:
					err := dbBuild.Finish(s)
					Expect(err).ToNot(HaveOccurred())
				}

				_, err := defaultTeam.CreateContainer(dbWorker.Name(), db.NewBuildStepContainerOwner(dbBuild.ID(), atc.PlanID(4)), db.ContainerMetadata{})
				Expect(err).ToNot(HaveOccurred())

				_, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				_, err = workerLifecycle.DeleteFinishedRetiringWorkers()
				Expect(err).ToNot(HaveOccurred())

				_, found, err = workerFactory.GetWorker(atcWorker.Name)
				Expect(err).ToNot(HaveOccurred())
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
					}, db.ConfigVersion(0), db.PipelineUnpaused)
					Expect(err).ToNot(HaveOccurred())
					Expect(created).To(BeTrue())

					job, found, err := pipeline.Job("some-job")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					dbBuild, err = job.CreateBuild()
					Expect(err).ToNot(HaveOccurred())
				})

				DescribeTable("with builds that are",
					ItRetiresWorkerWithState,
					Entry("pending", db.BuildStatusPending, true),
					Entry("started", db.BuildStatusStarted, true),
					Entry("aborted", db.BuildStatusAborted, false),
					Entry("succeeded", db.BuildStatusSucceeded, false),
					Entry("failed", db.BuildStatusFailed, false),
					Entry("errored", db.BuildStatusErrored, false),
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
					}, db.ConfigVersion(0), db.PipelineUnpaused)
					Expect(err).ToNot(HaveOccurred())
					Expect(created).To(BeTrue())

					job, found, err := pipeline.Job("some-job")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					dbBuild, err = job.CreateBuild()
					Expect(err).ToNot(HaveOccurred())
				})

				DescribeTable("with builds that are",
					ItRetiresWorkerWithState,
					Entry("pending", db.BuildStatusPending, false),
					Entry("started", db.BuildStatusStarted, false),
					Entry("aborted", db.BuildStatusAborted, false),
					Entry("succeeded", db.BuildStatusSucceeded, false),
					Entry("failed", db.BuildStatusFailed, false),
					Entry("errored", db.BuildStatusErrored, false),
				)
			})

			Context("when worker has one-off build", func() {
				BeforeEach(func() {
					var err error
					dbBuild, err = defaultTeam.CreateOneOffBuild()
					Expect(err).ToNot(HaveOccurred())
				})

				DescribeTable("with builds that are",
					ItRetiresWorkerWithState,
					Entry("pending", db.BuildStatusPending, true),
					Entry("started", db.BuildStatusStarted, true),
					Entry("aborted", db.BuildStatusAborted, false),
					Entry("succeeded", db.BuildStatusSucceeded, false),
					Entry("failed", db.BuildStatusFailed, false),
					Entry("errored", db.BuildStatusErrored, false),
				)
			})
		})
	})

	Describe("LandFinishedLandingWorkers", func() {
		var (
			dbWorker db.Worker
			dbBuild  db.Build
		)

		JustBeforeEach(func() {
			var err error
			dbWorker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when worker is not landing", func() {
			JustBeforeEach(func() {
				var err error
				atcWorker.State = string(db.WorkerStateRunning)
				dbWorker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not land worker", func() {
				_, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				landedWorkers, err := workerLifecycle.LandFinishedLandingWorkers()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(landedWorkers)).To(Equal(0))

				foundWorker, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(foundWorker.State()).To(Equal(db.WorkerStateRunning))
			})
		})

		Context("when worker is landing", func() {
			BeforeEach(func() {
				atcWorker.State = string(db.WorkerStateLanding)
			})

			Context("when the worker does not have any running builds", func() {
				It("lands worker", func() {
					_, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					landedWorkers, err := workerLifecycle.LandFinishedLandingWorkers()
					Expect(err).ToNot(HaveOccurred())
					Expect(len(landedWorkers)).To(Equal(1))
					Expect(landedWorkers[0]).To(Equal(atcWorker.Name))

					foundWorker, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(foundWorker.State()).To(Equal(db.WorkerStateLanded))
				})

				It("clears out the garden/baggageclaim addresses", func() {
					var (
						beforegardenAddr      sql.NullString
						beforeBaggagaClaimUrl sql.NullString
						aftergardenAddr       sql.NullString
						afterBaggagaClaimUrl  sql.NullString
						found                 bool
						err                   error
					)

					worker, found, err = workerFactory.GetWorker(atcWorker.Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					err = dbConn.QueryRow("SELECT addr, baggageclaim_url FROM workers WHERE name = '"+atcWorker.Name+"'").Scan(&beforegardenAddr,
						&beforeBaggagaClaimUrl,
					)
					Expect(err).ToNot(HaveOccurred())

					Expect(beforegardenAddr.Valid).To(BeTrue())
					Expect(beforeBaggagaClaimUrl.Valid).To(BeTrue())

					err = worker.Land()
					Expect(err).ToNot(HaveOccurred())
					landedWorkers, err := workerLifecycle.LandFinishedLandingWorkers()
					Expect(err).ToNot(HaveOccurred())
					Expect(len(landedWorkers)).To(Equal(1))
					Expect(landedWorkers[0]).To(Equal(atcWorker.Name))

					err = dbConn.QueryRow("SELECT addr, baggageclaim_url FROM workers WHERE name = '"+atcWorker.Name+"'").Scan(&aftergardenAddr,
						&afterBaggagaClaimUrl,
					)
					Expect(err).ToNot(HaveOccurred())

					Expect(aftergardenAddr.String).To(Equal(""))
					Expect(afterBaggagaClaimUrl.String).To(Equal(""))

				})
			})

			DescribeTable("land workers with builds that are",
				func(s db.BuildStatus, expectedState db.WorkerState) {
					dbBuild, err := defaultTeam.CreateOneOffBuild()
					Expect(err).ToNot(HaveOccurred())

					switch s {
					case db.BuildStatusPending:
					case db.BuildStatusStarted:
						_, err := dbBuild.Start("exec.v2", "{}", atc.Plan{})
						Expect(err).ToNot(HaveOccurred())
					default:
						err := dbBuild.Finish(s)
						Expect(err).ToNot(HaveOccurred())
					}

					_, err = defaultTeam.CreateContainer(dbWorker.Name(), db.NewBuildStepContainerOwner(dbBuild.ID(), atc.PlanID(4)), db.ContainerMetadata{})
					Expect(err).ToNot(HaveOccurred())

					_, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					_, err = workerLifecycle.LandFinishedLandingWorkers()
					Expect(err).ToNot(HaveOccurred())

					foundWorker, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(foundWorker.State()).To(Equal(expectedState))
				},
				Entry("pending", db.BuildStatusPending, db.WorkerStateLanding),
				Entry("started", db.BuildStatusStarted, db.WorkerStateLanding),
				Entry("aborted", db.BuildStatusAborted, db.WorkerStateLanded),
				Entry("succeeded", db.BuildStatusSucceeded, db.WorkerStateLanded),
				Entry("failed", db.BuildStatusFailed, db.WorkerStateLanded),
				Entry("errored", db.BuildStatusErrored, db.WorkerStateLanded),
			)

			ItLandsWorkerWithExpectedState := func(s db.BuildStatus, expectedState db.WorkerState) {
				switch s {
				case db.BuildStatusPending:
				case db.BuildStatusStarted:
					_, err := dbBuild.Start("exec.v2", "{}", atc.Plan{})
					Expect(err).ToNot(HaveOccurred())
				default:
					err := dbBuild.Finish(s)
					Expect(err).ToNot(HaveOccurred())
				}

				_, err := defaultTeam.CreateContainer(dbWorker.Name(), db.NewBuildStepContainerOwner(dbBuild.ID(), atc.PlanID(4)), db.ContainerMetadata{})
				Expect(err).ToNot(HaveOccurred())

				_, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				_, err = workerLifecycle.LandFinishedLandingWorkers()
				Expect(err).ToNot(HaveOccurred())

				foundWorker, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).ToNot(HaveOccurred())
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
					}, db.ConfigVersion(0), db.PipelineUnpaused)
					Expect(err).ToNot(HaveOccurred())
					Expect(created).To(BeTrue())

					job, found, err := pipeline.Job("some-job")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					dbBuild, err = job.CreateBuild()
					Expect(err).ToNot(HaveOccurred())
				})

				DescribeTable("with builds that are",
					ItLandsWorkerWithExpectedState,
					Entry("pending", db.BuildStatusPending, db.WorkerStateLanding),
					Entry("started", db.BuildStatusStarted, db.WorkerStateLanding),
					Entry("aborted", db.BuildStatusAborted, db.WorkerStateLanded),
					Entry("succeeded", db.BuildStatusSucceeded, db.WorkerStateLanded),
					Entry("failed", db.BuildStatusFailed, db.WorkerStateLanded),
					Entry("errored", db.BuildStatusErrored, db.WorkerStateLanded),
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
					}, db.ConfigVersion(0), db.PipelineUnpaused)
					Expect(err).ToNot(HaveOccurred())
					Expect(created).To(BeTrue())

					job, found, err := pipeline.Job("some-job")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					dbBuild, err = job.CreateBuild()
					Expect(err).ToNot(HaveOccurred())
				})

				DescribeTable("with builds that are",
					ItLandsWorkerWithExpectedState,
					Entry("pending", db.BuildStatusPending, db.WorkerStateLanded),
					Entry("started", db.BuildStatusStarted, db.WorkerStateLanded),
					Entry("aborted", db.BuildStatusAborted, db.WorkerStateLanded),
					Entry("succeeded", db.BuildStatusSucceeded, db.WorkerStateLanded),
					Entry("failed", db.BuildStatusFailed, db.WorkerStateLanded),
					Entry("errored", db.BuildStatusErrored, db.WorkerStateLanded),
				)
			})

			Context("when worker has one-off build", func() {
				BeforeEach(func() {
					var err error
					dbBuild, err = defaultTeam.CreateOneOffBuild()
					Expect(err).ToNot(HaveOccurred())
				})

				DescribeTable("with builds that are",
					ItLandsWorkerWithExpectedState,
					Entry("pending", db.BuildStatusPending, db.WorkerStateLanding),
					Entry("started", db.BuildStatusStarted, db.WorkerStateLanding),
					Entry("aborted", db.BuildStatusAborted, db.WorkerStateLanded),
					Entry("succeeded", db.BuildStatusSucceeded, db.WorkerStateLanded),
					Entry("failed", db.BuildStatusFailed, db.WorkerStateLanded),
					Entry("errored", db.BuildStatusErrored, db.WorkerStateLanded),
				)
			})
		})
	})
})
