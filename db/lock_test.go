package db_test

import (
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Locks", func() {
	var (
		dbConn   db.Conn
		listener *pq.Listener

		pipelineDBFactory db.PipelineDBFactory
		teamDBFactory     db.TeamDBFactory
		lockFactory       db.LockFactory
		sqlDB             *db.SQLDB

		lock       db.Lock
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
		lockFactory = db.NewLockFactory(postgresRunner.OpenPgx())
		sqlDB = db.NewSQL(dbConn, bus, lockFactory)
		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus, lockFactory)

		teamDBFactory = db.NewTeamDBFactory(dbConn, bus, lockFactory)
		teamDB = teamDBFactory.GetTeamDB(atc.DefaultTeamName)

		_, err := sqlDB.CreateTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())
		teamDB := teamDBFactory.GetTeamDB("some-team")

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

		savedPipeline, _, err := teamDB.SaveConfig("pipeline-name", pipelineConfig, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDB = pipelineDBFactory.Build(savedPipeline)
		lock = lockFactory.NewLock(logger, 42)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())

		lock.Release()
	})

	Describe("leases in general", func() {
		It("Acquire can only obtain lock once", func() {
			acquired, err := lock.Acquire()
			Expect(err).NotTo(HaveOccurred())
			Expect(acquired).To(BeTrue())

			acquired, err = lock.Acquire()
			Expect(err).NotTo(HaveOccurred())
			Expect(acquired).To(BeFalse())
		})

		It("Release is idempotent", func() {
			acquired, err := lock.Acquire()
			Expect(err).NotTo(HaveOccurred())
			Expect(acquired).To(BeTrue())

			err = lock.Release()
			Expect(err).NotTo(HaveOccurred())

			err = lock.Release()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("taking out a lock on pipeline scheduling", func() {
		Context("when it has been scheduled recently", func() {
			It("does not get the lock", func() {
				lock, acquired, err := pipelineDB.AcquireSchedulingLock(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				lock.Release()

				_, acquired, err = pipelineDB.AcquireSchedulingLock(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeFalse())
			})
		})

		Context("when there has not been any scheduling recently", func() {
			It("gets and keeps the lock and stops others from getting it", func() {
				lock, acquired, err := pipelineDB.AcquireSchedulingLock(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				Consistently(func() bool {
					_, acquired, err = pipelineDB.AcquireSchedulingLock(logger, 1*time.Second)
					Expect(err).NotTo(HaveOccurred())

					return acquired
				}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

				lock.Release()

				time.Sleep(time.Second)

				newLease, acquired, err := pipelineDB.AcquireSchedulingLock(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				newLease.Release()
			})
		})
	})

	Describe("GetNextPendingBuild", func() {
		Context("when a build is created and then the lock is acquired", func() {
			BeforeEach(func() {
				_, err := pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				var acquired bool
				lock, acquired, err = pipelineDB.AcquireResourceCheckingForJobLock(logger, "some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())
			})

			It("returns the build while the lock is acquired", func() {
				_, found, err := pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("when the lock is acquired and then a build is created", func() {
			BeforeEach(func() {
				var err error
				var acquired bool
				lock, acquired, err = pipelineDB.AcquireResourceCheckingForJobLock(logger, "some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				_, err = pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the build only after the lock is broken", func() {
				_, found, err := pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())

				lock.Release()

				_, found, err = pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("still returns the build after the lock is broken and reacquired", func() {
				lock.Release()

				_, acquired, err := pipelineDB.AcquireResourceCheckingForJobLock(logger, "some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				_, found, err := pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			Context("when someone else attempts to acquire the lock", func() {
				It("still doesn't return the build before the lock is broken", func() {
					_, acquired, err := pipelineDB.AcquireResourceCheckingForJobLock(logger, "some-job")
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeFalse())

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

		Context("when the lock is acquired and then a build is created", func() {
			BeforeEach(func() {
				var err error
				var acquired bool
				lock, acquired, err = pipelineDB.AcquireResourceCheckingForJobLock(logger, "some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				_, err = pipelineDB.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
			})

			It("doesn't create another build", func() {
				err := pipelineDB.EnsurePendingBuildExists("some-job")
				Expect(err).NotTo(HaveOccurred())

				lock.Release()

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

		Context("when the lock is acquired and no pending build exists", func() {
			var lock db.Lock
			BeforeEach(func() {
				var err error
				var acquired bool
				lock, acquired, err = pipelineDB.AcquireResourceCheckingForJobLock(logger, "some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())
			})

			It("creates a build", func() {
				err := pipelineDB.EnsurePendingBuildExists("some-job")
				Expect(err).NotTo(HaveOccurred())

				lock.Release()

				_, found, err := pipelineDB.GetNextPendingBuild("some-job")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})
	})

	Describe("AcquireResourceCheckingLock", func() {
		BeforeEach(func() {
			_, _, err := pipelineDB.GetResource("some-resource")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there has been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets the lock", func() {
					lock, acquired, err := pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()

					lock, acquired, err = pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})
			})

			Context("when not acquiring immediately", func() {
				It("does not get the lock", func() {
					lock, acquired, err := pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()

					lock, acquired, err = pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeFalse())
				})
			})
		})

		Context("when there has not been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets and keeps the lock and stops others from periodically getting it", func() {
					lock, acquired, err := pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, false)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					lock, acquired, err = pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})

				It("gets and keeps the lock and stops others from immediately getting it", func() {
					lock, acquired, err := pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, true)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					lock, acquired, err = pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})
			})

			Context("when not acquiring immediately", func() {
				It("gets and keeps the lock and stops others from periodically getting it", func() {
					lock, acquired, err := pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, false)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					lock, acquired, err = pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})

				It("gets and keeps the lock and stops others from immediately getting it", func() {
					lock, acquired, err := pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, true)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					lock, acquired, err = pipelineDB.AcquireResourceCheckingLock(logger, "some-resource", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})
			})
		})
	})

	Describe("AcquireResourceTypeCheckingLock", func() {
		BeforeEach(func() {
			_, found, err := pipelineDB.GetResourceType("some-resource-type")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when there has been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets the lock", func() {
					var acquired bool
					var err error
					lock, acquired, err = pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()

					lock, acquired, err = pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())
				})
			})

			Context("when not acquiring immediately", func() {
				It("does not get the lock", func() {
					var acquired bool
					var err error
					lock, acquired, err = pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()

					_, acquired, err = pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeFalse())
				})
			})
		})

		Context("when there has not been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets and keeps the lock and stops others from periodically getting it", func() {
					lock, acquired, err := pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, false)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					newLease, acquired, err := pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					newLease.Release()
				})

				It("gets and keeps the lock and stops others from immediately getting it", func() {
					lock, acquired, err := pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, true)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					newLease, acquired, err := pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					newLease.Release()
				})
			})

			Context("when not acquiring immediately", func() {
				It("gets and keeps the lock and stops others from periodically getting it", func() {
					lock, acquired, err := pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, false)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					newLease, acquired, err := pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					newLease.Release()
				})

				It("gets and keeps the lock and stops others from immediately getting it", func() {
					lock, acquired, err := pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, true)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					newLease, acquired, err := pipelineDB.AcquireResourceTypeCheckingLock(logger, "some-resource-type", 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					newLease.Release()
				})
			})
		})
	})

	Describe("taking out a lock on build tracking", func() {
		var build db.Build

		BeforeEach(func() {
			var err error
			build, err = teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when something has been tracking it recently", func() {
			It("does not get the lock", func() {
				lock, acquired, err := build.AcquireTrackingLock(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				lock.Release()

				_, acquired, err = build.AcquireTrackingLock(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeFalse())
			})
		})

		Context("when there has not been any tracking recently", func() {
			It("gets and keeps the lock and stops others from getting it", func() {
				lock, acquired, err := build.AcquireTrackingLock(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				Consistently(func() bool {
					_, acquired, err = build.AcquireTrackingLock(logger, 1*time.Second)
					Expect(err).NotTo(HaveOccurred())

					return acquired
				}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

				lock.Release()

				time.Sleep(time.Second)

				newLease, acquired, err := build.AcquireTrackingLock(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				newLease.Release()
			})
		})
	})

	Describe("GetLock", func() {
		Context("when something got the lock recently", func() {
			It("does not get the lock", func() {
				lock, acquired, err := sqlDB.GetLock(logger, "some-task-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				_, acquired, err = sqlDB.GetLock(logger, "some-task-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeFalse())

				lock.Release()
			})
		})

		Context("when no one got the lock recently", func() {
			It("gets and keeps the lock and stops others from getting it", func() {
				lock, acquired, err := sqlDB.GetLock(logger, "some-task-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				Consistently(func() bool {
					_, acquired, err = sqlDB.GetLock(logger, "some-task-name")
					Expect(err).NotTo(HaveOccurred())

					return acquired
				}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

				lock.Release()
			})
		})

		Context("when something got a different lock recently", func() {
			It("still gets the lock", func() {
				lock, acquired, err := sqlDB.GetLock(logger, "some-other-task-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				lock.Release()

				newLease, acquired, err := sqlDB.GetLock(logger, "some-task-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				newLease.Release()
			})
		})
	})
})
