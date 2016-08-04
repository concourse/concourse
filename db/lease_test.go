package db_test

import (
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Leases", func() {
	var (
		dbConn   db.Conn
		listener *pq.Listener

		pipelineDBFactory db.PipelineDBFactory
		teamDBFactory     db.TeamDBFactory
		sqlDB             *db.SQLDB

		pipelineDB db.PipelineDB
		teamDB     db.TeamDB

		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		logger = lagertest.NewTestLogger("test")
		sqlDB = db.NewSQL(dbConn, bus)
		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus)

		teamDBFactory = db.NewTeamDBFactory(dbConn, bus)
		teamDB = teamDBFactory.GetTeamDB(atc.DefaultTeamName)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	pipelineConfig := atc.Config{
		Resources: atc.ResourceConfigs{
			{
				Name: "some-resource",
				Type: "some-type",
				Source: atc.Source{
					"source-config": "some-value",
				},
			},
		},
		ResourceTypes: atc.ResourceTypes{
			{
				Name: "some-resource-type",
				Type: "some-type",
				Source: atc.Source{
					"source-config": "some-value",
				},
			},
		},
		Jobs: atc.JobConfigs{
			{
				Name: "some-job",
			},
		},
	}

	BeforeEach(func() {
		_, err := sqlDB.CreateTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())
		teamDB := teamDBFactory.GetTeamDB("some-team")
		savedPipeline, _, err := teamDB.SaveConfig("pipeline-name", pipelineConfig, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDB = pipelineDBFactory.Build(savedPipeline)
	})

	Describe("leases in general", func() {
		Context("when its Break method is called more than once", func() {
			It("only calls breakFunc the first time", func() {
				goodResult := new(dbfakes.FakeSqlResult)
				goodResult.RowsAffectedReturns(1, nil)

				leaseTester := new(dbfakes.FakeLeaseTester)
				leaseTester.AttemptSignReturns(goodResult, nil)
				leaseTester.HeartbeatReturns(goodResult, nil)

				lease, leased, err := db.NewLeaseForTesting(dbConn, logger, leaseTester, 1*time.Minute)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				Expect(leaseTester.BreakCallCount()).To(BeZero())
				lease.Break()
				Expect(leaseTester.BreakCallCount()).To(Equal(1))

				lease.Break()
				Expect(leaseTester.BreakCallCount()).To(Equal(1))

				lease.Break()
				Expect(leaseTester.BreakCallCount()).To(Equal(1))
			})
		})
	})

	Describe("taking out a lease on pipeline scheduling", func() {
		Context("when it has been scheduled recently", func() {
			It("does not get the lease", func() {
				lease, leased, err := pipelineDB.LeaseScheduling(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				lease.Break()

				_, leased, err = pipelineDB.LeaseScheduling(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeFalse())
			})
		})

		Context("when there has not been any scheduling recently", func() {
			It("gets and keeps the lease and stops others from getting it", func() {
				lease, leased, err := pipelineDB.LeaseScheduling(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				Consistently(func() bool {
					_, leased, err = pipelineDB.LeaseScheduling(logger, 1*time.Second)
					Expect(err).NotTo(HaveOccurred())

					return leased
				}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

				lease.Break()

				time.Sleep(time.Second)

				newLease, leased, err := pipelineDB.LeaseScheduling(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				newLease.Break()
			})
		})
	})

	Describe("GetNextPendingBuild", func() {
		Context("when a build is created and then the lease is acquired", func() {
			BeforeEach(func() {
				_, err := pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				_, leased, err := pipelineDB.LeaseResourceCheckingForJob(logger, "some-job", 1*time.Minute)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())
			})

			It("returns the build while the lease is acquired", func() {
				_, found, err := pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("when the lease is acquired and then a build is created", func() {
			var lease db.Lease
			BeforeEach(func() {
				var err error
				var leased bool
				lease, leased, err = pipelineDB.LeaseResourceCheckingForJob(logger, "some-job", 1*time.Minute)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				_, err = pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the build only after the lease is broken", func() {
				_, found, err := pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())

				lease.Break()

				_, found, err = pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("still returns the build after the lease is broken and reacquired", func() {
				lease.Break()

				_, leased, err := pipelineDB.LeaseResourceCheckingForJob(logger, "some-job", 1*time.Minute)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				_, found, err := pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			Context("when someone else attempts to acquire the lease", func() {
				It("still doesn't return the build before the lease is broken", func() {
					_, leased, err := pipelineDB.LeaseResourceCheckingForJob(logger, "some-job", 1*time.Minute)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeFalse())

					_, found, err := pipelineDB.GetNextPendingBuild("some-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeFalse())
				})
			})
		})
	})

	Describe("EnsurePendingBuildExists", func() {
		Context("when only a started build exists", func() {
			BeforeEach(func() {
				build1, err := pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				started, err := build1.Start("some-engine", "some-metadata")
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())
			})

			It("creates a build", func() {
				err := pipelineDB.EnsurePendingBuildExists("some-job")
				Expect(err).NotTo(HaveOccurred())

				_, found, err := pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("doesn't create another build the second time it's called", func() {
				err := pipelineDB.EnsurePendingBuildExists("some-job")
				Expect(err).NotTo(HaveOccurred())

				err = pipelineDB.EnsurePendingBuildExists("some-job")
				Expect(err).NotTo(HaveOccurred())

				build2, found, err := pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				started, err := build2.Start("some-engine", "some-metadata")
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())

				_, found, err = pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the lease is acquired and then a build is created", func() {
			var lease db.Lease
			BeforeEach(func() {
				var err error
				var leased bool
				lease, leased, err = pipelineDB.LeaseResourceCheckingForJob(logger, "some-job", 1*time.Minute)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				_, err = pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
			})

			It("doesn't create another build", func() {
				err := pipelineDB.EnsurePendingBuildExists("some-job")
				Expect(err).NotTo(HaveOccurred())

				lease.Break()

				build1, found, err := pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				started, err := build1.Start("some-engine", "some-metadata")
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())

				_, found, err = pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the lease is acquired and no pending build exists", func() {
			var lease db.Lease
			BeforeEach(func() {
				var err error
				var leased bool
				lease, leased, err = pipelineDB.LeaseResourceCheckingForJob(logger, "some-job", 1*time.Minute)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())
			})

			It("creates a build", func() {
				err := pipelineDB.EnsurePendingBuildExists("some-job")
				Expect(err).NotTo(HaveOccurred())

				lease.Break()

				_, found, err := pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})
	})

	Describe("LeaseResourceChecking", func() {
		BeforeEach(func() {
			_, _, err := pipelineDB.GetResource("some-resource")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there has been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets the lease", func() {
					lease, leased, err := pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					lease.Break()

					lease, leased, err = pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					lease.Break()
				})
			})

			Context("when not acquiring immediately", func() {
				It("does not get the lease", func() {
					lease, leased, err := pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					lease.Break()

					_, leased, err = pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeFalse())
				})
			})
		})

		Context("when there has not been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets and keeps the lease and stops others from periodically getting it", func() {
					lease, leased, err := pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					Consistently(func() bool {
						_, leased, err = pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, false)
						Expect(err).NotTo(HaveOccurred())

						return leased
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lease.Break()

					time.Sleep(time.Second)

					newLease, leased, err := pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					newLease.Break()
				})

				It("gets and keeps the lease and stops others from immediately getting it", func() {
					lease, leased, err := pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					Consistently(func() bool {
						_, leased, err = pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, true)
						Expect(err).NotTo(HaveOccurred())

						return leased
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lease.Break()

					time.Sleep(time.Second)

					newLease, leased, err := pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					newLease.Break()
				})
			})

			Context("when not acquiring immediately", func() {
				It("gets and keeps the lease and stops others from periodically getting it", func() {
					lease, leased, err := pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					Consistently(func() bool {
						_, leased, err = pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, false)
						Expect(err).NotTo(HaveOccurred())

						return leased
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lease.Break()

					time.Sleep(time.Second)

					newLease, leased, err := pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					newLease.Break()
				})

				It("gets and keeps the lease and stops others from immediately getting it", func() {
					lease, leased, err := pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					Consistently(func() bool {
						_, leased, err = pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, true)
						Expect(err).NotTo(HaveOccurred())

						return leased
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lease.Break()

					time.Sleep(time.Second)

					newLease, leased, err := pipelineDB.LeaseResourceChecking(logger, "some-resource", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					newLease.Break()
				})
			})
		})
	})

	Describe("LeaseResourceTypeChecking", func() {
		BeforeEach(func() {
			_, found, err := pipelineDB.GetResourceType("some-resource-type")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when there has been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets the lease", func() {
					lease, leased, err := pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					lease.Break()

					lease, leased, err = pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					lease.Break()
				})
			})

			Context("when not acquiring immediately", func() {
				It("does not get the lease", func() {
					lease, leased, err := pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					lease.Break()

					_, leased, err = pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeFalse())
				})
			})
		})

		Context("when there has not been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets and keeps the lease and stops others from periodically getting it", func() {
					lease, leased, err := pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					Consistently(func() bool {
						_, leased, err = pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, false)
						Expect(err).NotTo(HaveOccurred())

						return leased
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lease.Break()

					time.Sleep(time.Second)

					newLease, leased, err := pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					newLease.Break()
				})

				It("gets and keeps the lease and stops others from immediately getting it", func() {
					lease, leased, err := pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					Consistently(func() bool {
						_, leased, err = pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, true)
						Expect(err).NotTo(HaveOccurred())

						return leased
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lease.Break()

					time.Sleep(time.Second)

					newLease, leased, err := pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					newLease.Break()
				})
			})

			Context("when not acquiring immediately", func() {
				It("gets and keeps the lease and stops others from periodically getting it", func() {
					lease, leased, err := pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					Consistently(func() bool {
						_, leased, err = pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, false)
						Expect(err).NotTo(HaveOccurred())

						return leased
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lease.Break()

					time.Sleep(time.Second)

					newLease, leased, err := pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					newLease.Break()
				})

				It("gets and keeps the lease and stops others from immediately getting it", func() {
					lease, leased, err := pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					Consistently(func() bool {
						_, leased, err = pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, true)
						Expect(err).NotTo(HaveOccurred())

						return leased
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lease.Break()

					time.Sleep(time.Second)

					newLease, leased, err := pipelineDB.LeaseResourceTypeChecking(logger, "some-resource-type", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(leased).To(BeTrue())

					newLease.Break()
				})
			})
		})
	})

	Describe("taking out a lease on build tracking", func() {
		var build db.Build

		BeforeEach(func() {
			var err error
			build, err = teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when something has been tracking it recently", func() {
			It("does not get the lease", func() {
				lease, leased, err := build.LeaseTracking(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				lease.Break()

				_, leased, err = build.LeaseTracking(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeFalse())
			})
		})

		Context("when there has not been any tracking recently", func() {
			It("gets and keeps the lease and stops others from getting it", func() {
				lease, leased, err := build.LeaseTracking(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				Consistently(func() bool {
					_, leased, err = build.LeaseTracking(logger, 1*time.Second)
					Expect(err).NotTo(HaveOccurred())

					return leased
				}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

				lease.Break()

				time.Sleep(time.Second)

				newLease, leased, err := build.LeaseTracking(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				newLease.Break()
			})
		})
	})

	Describe("taking out a lease on cache invalidation", func() {
		Context("when something got the lease recently", func() {
			It("does not get the lease", func() {
				lease, leased, err := sqlDB.GetLease(logger, "some-task-name", 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				lease.Break()

				_, leased, err = sqlDB.GetLease(logger, "some-task-name", 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeFalse())
			})
		})

		Context("when no one got the lease recently", func() {
			It("gets and keeps the lease and stops others from getting it", func() {
				lease, leased, err := sqlDB.GetLease(logger, "some-task-name", 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				Consistently(func() bool {
					_, leased, err = sqlDB.GetLease(logger, "some-task-name", 1*time.Second)
					Expect(err).NotTo(HaveOccurred())

					return leased
				}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

				lease.Break()

				time.Sleep(time.Second)

				newLease, leased, err := sqlDB.GetLease(logger, "some-task-name", 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				newLease.Break()
			})
		})

		Context("when something got a different lease recently", func() {
			It("still gets the lease", func() {
				lease, leased, err := sqlDB.GetLease(logger, "some-other-task-name", 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				lease.Break()

				newLease, leased, err := sqlDB.GetLease(logger, "some-task-name", 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(leased).To(BeTrue())

				newLease.Break()
			})
		})
	})
})
