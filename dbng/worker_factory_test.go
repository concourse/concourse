package dbng_test

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/types"
)

var _ = Describe("WorkerFactory", func() {
	var (
		atcWorker atc.Worker
		worker    *dbng.Worker
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

	Describe("SaveWorker", func() {
		Context("the worker already exists", func() {
			BeforeEach(func() {
				var err error
				worker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("removes old worker resource type", func() {
				atcWorker.ResourceTypes = []atc.WorkerResourceType{
					{
						Type:    "other-resource-type",
						Image:   "other-image",
						Version: "other-version",
					},
				}

				_, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())

				tx, err := dbConn.Begin()
				Expect(err).NotTo(HaveOccurred())
				defer tx.Rollback()

				var count int
				err = psql.Select("count(*)").
					From("worker_base_resource_types").
					Where(sq.Eq{"worker_name": "some-name"}).
					RunWith(tx).
					QueryRow().Scan(&count)
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(1))
			})

			Context("the worker is in stalled state", func() {
				BeforeEach(func() {
					_, err = workerFactory.StallWorker(worker.Name)
					Expect(err).NotTo(HaveOccurred())
				})

				It("repopulates the garden address", func() {
					savedWorker, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
					Expect(err).NotTo(HaveOccurred())
					Expect(savedWorker.Name).To(Equal("some-name"))
					Expect(*savedWorker.GardenAddr).To(Equal("some-garden-addr"))
					Expect(savedWorker.State).To(Equal(dbng.WorkerStateRunning))
				})
			})
		})

		Context("no worker with same name exists", func() {
			It("saves worker", func() {
				savedWorker, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
				Expect(savedWorker.Name).To(Equal("some-name"))
				Expect(*savedWorker.GardenAddr).To(Equal("some-garden-addr"))
				Expect(savedWorker.State).To(Equal(dbng.WorkerStateRunning))
			})

			It("saves worker resource types as base resource types", func() {
				_, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())

				tx, err := dbConn.Begin()
				Expect(err).NotTo(HaveOccurred())
				defer tx.Rollback()

				var count int
				err = psql.Select("count(*)").
					From("worker_base_resource_types").
					Where(sq.Eq{"worker_name": "some-name"}).
					RunWith(tx).
					QueryRow().Scan(&count)
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(2))
			})
		})
	})

	Describe("SaveTeamWorker", func() {
		var (
			otherTeam dbng.Team
		)

		BeforeEach(func() {
			var err error
			otherTeam, err = teamFactory.CreateTeam("otherTeam")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("the worker already exists", func() {
			Context("the worker is not in stalled state", func() {
				Context("the team_id of the new worker is the same", func() {
					BeforeEach(func() {
						_, err := defaultTeam.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).NotTo(HaveOccurred())
					})

					It("overwrites all the data", func() {
						atcWorker.GardenAddr = "new-garden-addr"
						savedWorker, err := defaultTeam.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).NotTo(HaveOccurred())
						Expect(savedWorker.Name).To(Equal("some-name"))
						Expect(*savedWorker.GardenAddr).To(Equal("new-garden-addr"))
						Expect(savedWorker.State).To(Equal(dbng.WorkerStateRunning))
					})
				})

				Context("the team_id of the new worker is different", func() {
					BeforeEach(func() {
						_, err := otherTeam.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).NotTo(HaveOccurred())
					})

					It("errors", func() {
						_, err := defaultTeam.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).To(HaveOccurred())
					})
				})
			})
		})
	})

	Describe("GetWorker", func() {
		Context("when the worker is present", func() {
			BeforeEach(func() {
				_, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("finds the worker", func() {
				foundWorker, found, err := workerFactory.GetWorker("some-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(foundWorker.Name).To(Equal("some-name"))
				Expect(*foundWorker.GardenAddr).To(Equal("some-garden-addr"))
				Expect(foundWorker.State).To(Equal(dbng.WorkerStateRunning))
				Expect(*foundWorker.BaggageclaimURL).To(Equal("some-bc-url"))
				Expect(foundWorker.HTTPProxyURL).To(Equal("some-http-proxy-url"))
				Expect(foundWorker.HTTPSProxyURL).To(Equal("some-https-proxy-url"))
				Expect(foundWorker.NoProxy).To(Equal("some-no-proxy"))
				Expect(foundWorker.ActiveContainers).To(Equal(140))
				Expect(foundWorker.ResourceTypes).To(Equal([]atc.WorkerResourceType{
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
				}))
				Expect(foundWorker.Platform).To(Equal("some-platform"))
				Expect(foundWorker.Tags).To(Equal([]string{"some", "tags"}))
				Expect(foundWorker.StartTime).To(Equal(int64(55)))
				Expect(foundWorker.State).To(Equal(dbng.WorkerStateRunning))
			})
		})

		Context("when the worker is not present", func() {
			It("returns false but no error", func() {
				foundWorker, found, err := workerFactory.GetWorker("some-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(foundWorker).To(BeNil())
			})
		})
	})

	Describe("Workers", func() {
		BeforeEach(func() {
			postgresRunner.Truncate()
		})

		Context("when there are workers", func() {
			BeforeEach(func() {
				_, err = workerFactory.SaveWorker(atcWorker, 0)
				Expect(err).NotTo(HaveOccurred())

				atcWorker.Name = "some-new-worker"
				atcWorker.GardenAddr = "some-other-garden-addr"
				atcWorker.BaggageclaimURL = "some-other-bc-url"
				_, err = workerFactory.SaveWorker(atcWorker, 0)
				Expect(err).NotTo(HaveOccurred())
			})

			It("finds them without error", func() {
				workers, err := workerFactory.Workers()
				addr := "some-garden-addr"
				bcURL := "some-bc-url"
				otherAddr := "some-other-garden-addr"
				otherBcURL := "some-other-bc-url"
				Expect(err).NotTo(HaveOccurred())
				Expect(len(workers)).To(Equal(2))
				Expect(workers).To(ConsistOf(
					&dbng.Worker{
						GardenAddr:       &addr,
						Name:             "some-name",
						State:            "running",
						BaggageclaimURL:  &bcURL,
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
						StartTime: 55,
					},
					&dbng.Worker{
						GardenAddr:       &otherAddr,
						Name:             "some-new-worker",
						State:            "running",
						BaggageclaimURL:  &otherBcURL,
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
						StartTime: 55,
					},
				))
			})
		})

		Context("when there are no workers", func() {
			It("returns an error", func() {
				workers, err := workerFactory.Workers()
				Expect(err).NotTo(HaveOccurred())
				Expect(workers).To(BeEmpty())
			})
		})
	})

	Describe("StallWorker", func() {
		Context("when the worker is present", func() {
			BeforeEach(func() {
				_, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("marks the worker as `stalled`", func() {
				foundWorker, err := workerFactory.StallWorker("some-name")
				Expect(err).NotTo(HaveOccurred())

				Expect(foundWorker.Name).To(Equal("some-name"))
				Expect(foundWorker.GardenAddr).To(BeNil())
				Expect(foundWorker.State).To(Equal(dbng.WorkerStateStalled))
			})
		})

		Context("when the worker is not present", func() {
			It("returns an error", func() {
				foundWorker, err := workerFactory.StallWorker("some-name")
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(dbng.ErrWorkerNotPresent))
				Expect(foundWorker).To(BeNil())
			})
		})
	})

	Describe("StallUnresponsiveWorkers", func() {
		Context("when the worker has heartbeated recently", func() {
			BeforeEach(func() {
				_, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("leaves the worker alone", func() {
				stalledWorkers, err := workerFactory.StallUnresponsiveWorkers()
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
				stalledWorkers, err := workerFactory.StallUnresponsiveWorkers()
				Expect(err).NotTo(HaveOccurred())

				Expect(len(stalledWorkers)).To(Equal(1))
				Expect(stalledWorkers[0].GardenAddr).To(BeNil())
				Expect(stalledWorkers[0].Name).To(Equal("some-name"))
				Expect(stalledWorkers[0].State).To(Equal(dbng.WorkerStateStalled))
			})
		})
	})

	Describe("DeleteFinishedRetiringWorkers", func() {
		var (
			dbWorker *dbng.Worker
			dbBuild  *dbng.Build
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

				err = workerFactory.DeleteFinishedRetiringWorkers()
				Expect(err).NotTo(HaveOccurred())

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

					err = workerFactory.DeleteFinishedRetiringWorkers()
					Expect(err).NotTo(HaveOccurred())

					_, found, err = workerFactory.GetWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeFalse())
				})
			})

			DescribeTable("deleting workers with builds that are",
				func(s dbng.BuildStatus, expectedExistence bool) {
					dbBuild, err := defaultTeam.CreateOneOffBuild()
					Expect(err).NotTo(HaveOccurred())

					tx, err := dbConn.Begin()
					Expect(err).NotTo(HaveOccurred())

					err = dbBuild.SaveStatus(tx, s)
					Expect(err).NotTo(HaveOccurred())

					err = tx.Commit()
					Expect(err).NotTo(HaveOccurred())

					_, err = defaultTeam.CreateBuildContainer(dbWorker, dbBuild, atc.PlanID(4), dbng.ContainerMetadata{})
					Expect(err).NotTo(HaveOccurred())

					_, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					err = workerFactory.DeleteFinishedRetiringWorkers()
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
				tx, err := dbConn.Begin()
				Expect(err).NotTo(HaveOccurred())

				err = dbBuild.SaveStatus(tx, s)
				Expect(err).NotTo(HaveOccurred())

				err = tx.Commit()
				Expect(err).NotTo(HaveOccurred())

				_, err = defaultTeam.CreateBuildContainer(dbWorker, dbBuild, atc.PlanID(4), dbng.ContainerMetadata{})
				Expect(err).NotTo(HaveOccurred())

				_, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				err = workerFactory.DeleteFinishedRetiringWorkers()
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
			dbWorker *dbng.Worker
			dbBuild  *dbng.Build
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

				err = workerFactory.LandFinishedLandingWorkers()
				Expect(err).NotTo(HaveOccurred())

				foundWorker, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(foundWorker.State).To(Equal(dbng.WorkerStateRunning))
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

					err = workerFactory.LandFinishedLandingWorkers()
					Expect(err).NotTo(HaveOccurred())

					foundWorker, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(foundWorker.State).To(Equal(dbng.WorkerStateLanded))
				})
			})

			DescribeTable("land workers with builds that are",
				func(s dbng.BuildStatus, expectedState dbng.WorkerState) {
					dbBuild, err := defaultTeam.CreateOneOffBuild()
					Expect(err).NotTo(HaveOccurred())

					tx, err := dbConn.Begin()
					Expect(err).NotTo(HaveOccurred())

					err = dbBuild.SaveStatus(tx, s)
					Expect(err).NotTo(HaveOccurred())

					err = tx.Commit()
					Expect(err).NotTo(HaveOccurred())

					_, err = defaultTeam.CreateBuildContainer(dbWorker, dbBuild, atc.PlanID(4), dbng.ContainerMetadata{})
					Expect(err).NotTo(HaveOccurred())

					_, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					err = workerFactory.LandFinishedLandingWorkers()
					Expect(err).NotTo(HaveOccurred())

					foundWorker, found, err := workerFactory.GetWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(foundWorker.State).To(Equal(expectedState))
				},
				Entry("pending", dbng.BuildStatusPending, dbng.WorkerStateLanding),
				Entry("started", dbng.BuildStatusStarted, dbng.WorkerStateLanding),
				Entry("aborted", dbng.BuildStatusAborted, dbng.WorkerStateLanded),
				Entry("succeeded", dbng.BuildStatusSucceeded, dbng.WorkerStateLanded),
				Entry("failed", dbng.BuildStatusFailed, dbng.WorkerStateLanded),
				Entry("errored", dbng.BuildStatusErrored, dbng.WorkerStateLanded),
			)

			ItLandsWorkerWithExpectedState := func(s dbng.BuildStatus, expectedState dbng.WorkerState) {
				tx, err := dbConn.Begin()
				Expect(err).NotTo(HaveOccurred())

				err = dbBuild.SaveStatus(tx, s)
				Expect(err).NotTo(HaveOccurred())

				err = tx.Commit()
				Expect(err).NotTo(HaveOccurred())

				_, err = defaultTeam.CreateBuildContainer(dbWorker, dbBuild, atc.PlanID(4), dbng.ContainerMetadata{})
				Expect(err).NotTo(HaveOccurred())

				_, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				err = workerFactory.LandFinishedLandingWorkers()
				Expect(err).NotTo(HaveOccurred())

				foundWorker, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(foundWorker.State).To(Equal(expectedState))
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

	Describe("LandWorker", func() {
		Context("when the worker is present", func() {
			BeforeEach(func() {
				_, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("marks the worker as `landing`", func() {
				foundWorker, err := workerFactory.LandWorker(atcWorker.Name)
				Expect(err).NotTo(HaveOccurred())

				Expect(foundWorker.Name).To(Equal(atcWorker.Name))
				Expect(foundWorker.State).To(Equal(dbng.WorkerStateLanding))
			})

			Context("when worker is already landed", func() {
				BeforeEach(func() {
					_, err := workerFactory.LandWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
					err = workerFactory.LandFinishedLandingWorkers()
					Expect(err).NotTo(HaveOccurred())
				})

				It("keeps worker state as landed", func() {
					foundWorker, err := workerFactory.LandWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())

					Expect(foundWorker.Name).To(Equal(atcWorker.Name))
					Expect(foundWorker.State).To(Equal(dbng.WorkerStateLanded))
				})
			})
		})

		Context("when the worker is not present", func() {
			It("returns an error", func() {
				foundWorker, err := workerFactory.LandWorker(atcWorker.Name)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(dbng.ErrWorkerNotPresent))
				Expect(foundWorker).To(BeNil())
			})
		})
	})

	Describe("RetireWorker", func() {
		Context("when the worker is present", func() {
			BeforeEach(func() {
				_, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("marks the worker as `retiring`", func() {
				foundWorker, err := workerFactory.RetireWorker(atcWorker.Name)
				Expect(err).NotTo(HaveOccurred())

				Expect(foundWorker.Name).To(Equal(atcWorker.Name))
				Expect(foundWorker.State).To(Equal(dbng.WorkerStateRetiring))
			})
		})

		Context("when the worker is not present", func() {
			It("returns an error", func() {
				foundWorker, err := workerFactory.RetireWorker(atcWorker.Name)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(dbng.ErrWorkerNotPresent))
				Expect(foundWorker).To(BeNil())
			})
		})
	})

	Describe("DeleteWorker", func() {
		BeforeEach(func() {
			_, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the record for the worker", func() {
			err := workerFactory.DeleteWorker(atcWorker.Name)
			Expect(err).NotTo(HaveOccurred())

			_, found, err := workerFactory.GetWorker(atcWorker.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("PruneWorker", func() {
		Context("when worker exists", func() {
			DescribeTable("worker in state",
				func(workerState string, errMatch GomegaMatcher) {
					_, err := workerFactory.SaveWorker(atc.Worker{
						Name:       "worker-to-prune",
						GardenAddr: "1.2.3.4",
						State:      workerState,
					}, 5*time.Minute)
					Expect(err).NotTo(HaveOccurred())

					err = workerFactory.PruneWorker("worker-to-prune")
					Expect(err).To(errMatch)
				},

				Entry("running", "running", Equal(dbng.ErrCannotPruneRunningWorker)),
				Entry("landing", "landing", BeNil()),
				Entry("landed", "landed", BeNil()),
				Entry("retiring", "retiring", BeNil()),
			)

			Context("when worker is stalled", func() {
				BeforeEach(func() {
					_, err := workerFactory.SaveWorker(atc.Worker{
						Name:       "worker-to-prune",
						GardenAddr: "1.2.3.4",
						State:      "running",
					}, 5*time.Minute)
					Expect(err).NotTo(HaveOccurred())

					_, err = workerFactory.StallWorker("worker-to-prune")
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not return error", func() {
					err := workerFactory.PruneWorker("worker-to-prune")
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Context("when worker does not exist", func() {
			It("raises ErrWorkerNotPresent", func() {
				err = workerFactory.PruneWorker("some-unknown-worker")
				Expect(err).To(Equal(dbng.ErrWorkerNotPresent))
			})
		})
	})

	Describe("HeartbeatWorker", func() {
		var (
			ttl              time.Duration
			epsilon          time.Duration
			activeContainers int
		)

		BeforeEach(func() {
			ttl = 5 * time.Minute
			epsilon = 30 * time.Second
			activeContainers = 0

			atcWorker.ActiveContainers = activeContainers
		})

		Context("when the worker is present", func() {
			JustBeforeEach(func() {
				_, err = workerFactory.SaveWorker(atcWorker, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates the expires field and the number of active containers", func() {
				atcWorker.ActiveContainers = 1

				foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
				Expect(err).NotTo(HaveOccurred())

				Expect(foundWorker.Name).To(Equal(atcWorker.Name))
				Expect(foundWorker.ExpiresIn - ttl).To(And(BeNumerically("<=", epsilon), BeNumerically(">", 0)))
				Expect(foundWorker.ActiveContainers).To(And(Not(Equal(activeContainers)), Equal(1)))
			})

			Context("when the current state is landing", func() {
				BeforeEach(func() {
					atcWorker.State = string(dbng.WorkerStateLanding)
				})

				It("keeps the state as landing", func() {
					foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
					Expect(err).NotTo(HaveOccurred())

					Expect(foundWorker.State).To(Equal(dbng.WorkerStateLanding))
				})
			})

			Context("when the current state is landed", func() {
				BeforeEach(func() {
					atcWorker.State = string(dbng.WorkerStateLanded)
				})

				It("keeps the state as landed", func() {
					foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
					Expect(err).NotTo(HaveOccurred())

					Expect(foundWorker.State).To(Equal(dbng.WorkerStateLanded))
				})
			})

			Context("when the current state is retiring", func() {
				BeforeEach(func() {
					atcWorker.State = string(dbng.WorkerStateRetiring)
				})

				It("keeps the state as landed", func() {
					foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
					Expect(err).NotTo(HaveOccurred())

					Expect(foundWorker.State).To(Equal(dbng.WorkerStateRetiring))
				})
			})

			Context("when the current state is running", func() {
				BeforeEach(func() {
					atcWorker.State = string(dbng.WorkerStateRunning)
				})

				It("keeps the state as running", func() {
					foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
					Expect(err).NotTo(HaveOccurred())

					Expect(foundWorker.State).To(Equal(dbng.WorkerStateRunning))
				})
			})

			Context("when the current state is stalled", func() {
				var stalledWorker *dbng.Worker

				JustBeforeEach(func() {
					stalledWorker, err = workerFactory.StallWorker(atcWorker.Name)
					Expect(err).NotTo(HaveOccurred())
				})

				It("sets the state as running", func() {
					Expect(stalledWorker.GardenAddr).To(BeNil())
					Expect(stalledWorker.BaggageclaimURL).To(BeNil())

					foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
					Expect(err).NotTo(HaveOccurred())

					Expect(foundWorker.GardenAddr).ToNot(BeNil())
					Expect(*foundWorker.GardenAddr).To(Equal("some-garden-addr"))
					Expect(foundWorker.BaggageclaimURL).ToNot(BeNil())
					Expect(*foundWorker.BaggageclaimURL).To(Equal("some-bc-url"))
					Expect(foundWorker.State).To(Equal(dbng.WorkerStateRunning))
				})
			})
		})

		Context("when the worker is not present", func() {
			It("returns an error", func() {
				foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(dbng.ErrWorkerNotPresent))
				Expect(foundWorker).To(BeNil())
			})
		})
	})
})
