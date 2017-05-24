package lock_test

import (
	"sync"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/atc/dbng"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Locks", func() {
	var (
		dbConn      db.Conn
		listener    *pq.Listener
		lockFactory lock.LockFactory
		sqlDB       *db.SQLDB

		dbLock lock.Lock

		pipeline    dbng.Pipeline
		team        dbng.Team
		teamFactory dbng.TeamFactory

		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.OpenDB())

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		logger = lagertest.NewTestLogger("test")

		lockFactory = lock.NewLockFactory(postgresRunner.OpenSingleton())
		sqlDB = db.NewSQL(dbConn, bus, lockFactory)

		dbLock = lockFactory.NewLock(logger, lock.LockID{42})

		dbngConn := postgresRunner.OpenConn()
		teamFactory = dbng.NewTeamFactory(dbngConn, lockFactory)

		var err error
		team, err = teamFactory.CreateTeam(atc.Team{Name: "team-name"})
		Expect(err).NotTo(HaveOccurred())

		pipeline, _, err = team.SavePipeline("some-pipeline", atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "some-job",
				},
			},
			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-base-resource-type",
					Source: atc.Source{
						"some": "source",
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
		}, dbng.ConfigVersion(0), dbng.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())

		dbLock.Release()
	})

	Describe("locks in general", func() {
		It("Acquire can only obtain lock once", func() {
			acquired, err := dbLock.Acquire()
			Expect(err).NotTo(HaveOccurred())
			Expect(acquired).To(BeTrue())

			acquired, err = dbLock.Acquire()
			Expect(err).NotTo(HaveOccurred())
			Expect(acquired).To(BeFalse())
		})

		It("Acquire accepts list of ids", func() {
			dbLock = lockFactory.NewLock(logger, lock.LockID{42, 56})

			Consistently(func() error {
				connCount := 3

				var anyError error
				var wg sync.WaitGroup
				wg.Add(connCount)

				for i := 0; i < connCount; i++ {
					go func() {
						defer wg.Done()

						_, err := dbLock.Acquire()
						if err != nil {
							anyError = err
						}

					}()
				}

				wg.Wait()

				return anyError
			}, 1500*time.Millisecond, 100*time.Millisecond).ShouldNot(HaveOccurred())

			dbLock = lockFactory.NewLock(logger, lock.LockID{56, 42})

			acquired, err := dbLock.Acquire()
			Expect(err).NotTo(HaveOccurred())
			Expect(acquired).To(BeTrue())

			acquired, err = dbLock.Acquire()
			Expect(err).NotTo(HaveOccurred())
			Expect(acquired).To(BeFalse())
		})

		Context("when another connection is holding the lock", func() {
			var lockFactory2 lock.LockFactory

			BeforeEach(func() {
				lockFactory2 = lock.NewLockFactory(postgresRunner.OpenSingleton())
			})

			It("does not acquire the lock", func() {
				acquired, err := dbLock.Acquire()
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				dbLock2 := lockFactory2.NewLock(logger, lock.LockID{42})
				acquired, err = dbLock2.Acquire()
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeFalse())

				dbLock.Release()
				dbLock2.Release()
			})

			It("acquires the locks once it is released", func() {
				acquired, err := dbLock.Acquire()
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				dbLock2 := lockFactory2.NewLock(logger, lock.LockID{42})
				acquired, err = dbLock2.Acquire()
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeFalse())

				dbLock.Release()

				acquired, err = dbLock2.Acquire()
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				dbLock2.Release()
			})
		})

		Context("when two locks are being acquired at the same time", func() {
			var lock1 lock.Lock
			var lock2 lock.Lock
			var fakeLockDB *lockfakes.FakeLockDB
			var acquiredLock2 chan struct{}
			var lock2Err error
			var lock2Acquired bool

			BeforeEach(func() {
				fakeLockDB = new(lockfakes.FakeLockDB)
				fakeLockFactory := lock.NewTestLockFactory(fakeLockDB)
				lock1 = fakeLockFactory.NewLock(logger, lock.LockID{57})
				lock2 = fakeLockFactory.NewLock(logger, lock.LockID{57})

				acquiredLock2 = make(chan struct{})
			})

			JustBeforeEach(func() {
				called := false
				readyToAcquire := make(chan struct{})

				fakeLockDB.AcquireStub = func(id lock.LockID) (bool, error) {
					if !called {
						called = true

						go func() {
							close(readyToAcquire)
							lock2Acquired, lock2Err = lock2.Acquire()
							close(acquiredLock2)
						}()

						<-readyToAcquire
					}

					return true, nil
				}
			})

			It("only acquires one of the locks", func() {
				acquired, err := lock1.Acquire()
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				<-acquiredLock2

				Expect(lock2Err).NotTo(HaveOccurred())
				Expect(lock2Acquired).To(BeFalse())
			})

			Context("when locks are being created on different lock factory (different db conn)", func() {
				BeforeEach(func() {
					fakeLockFactory2 := lock.NewTestLockFactory(fakeLockDB)
					lock2 = fakeLockFactory2.NewLock(logger, lock.LockID{57})
				})

				It("allows to acquire both locks", func() {
					acquired, err := lock1.Acquire()
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					<-acquiredLock2

					Expect(lock2Err).NotTo(HaveOccurred())
					Expect(lock2Acquired).To(BeTrue())
				})
			})
		})
	})

	Describe("taking out a lock on pipeline scheduling", func() {
		Context("when it has been scheduled recently", func() {
			It("does not get the lock", func() {
				lock, acquired, err := pipeline.AcquireSchedulingLock(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				lock.Release()

				_, acquired, err = pipeline.AcquireSchedulingLock(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeFalse())
			})
		})

		Context("when there has not been any scheduling recently", func() {
			It("gets and keeps the lock and stops others from getting it", func() {
				lock, acquired, err := pipeline.AcquireSchedulingLock(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				Consistently(func() bool {
					_, acquired, err = pipeline.AcquireSchedulingLock(logger, 1*time.Second)
					Expect(err).NotTo(HaveOccurred())

					return acquired
				}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

				lock.Release()

				time.Sleep(time.Second)

				newLock, acquired, err := pipeline.AcquireSchedulingLock(logger, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				newLock.Release()
			})
		})
	})

	Describe("taking out a lock on build tracking", func() {
		var build dbng.Build

		BeforeEach(func() {
			var err error
			build, err = team.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
		})

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

			newLock, acquired, err := build.AcquireTrackingLock(logger, 1*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(acquired).To(BeTrue())

			newLock.Release()
		})
	})

	Describe("GetTaskLock", func() {
		Context("when something got the lock recently", func() {
			It("does not get the lock", func() {
				lock, acquired, err := sqlDB.GetTaskLock(logger, "some-task-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				_, acquired, err = sqlDB.GetTaskLock(logger, "some-task-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeFalse())

				lock.Release()
			})
		})

		Context("when no one got the lock recently", func() {
			It("gets and keeps the lock and stops others from getting it", func() {
				lock, acquired, err := sqlDB.GetTaskLock(logger, "some-task-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				Consistently(func() bool {
					_, acquired, err = sqlDB.GetTaskLock(logger, "some-task-name")
					Expect(err).NotTo(HaveOccurred())

					return acquired
				}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

				lock.Release()
			})
		})

		Context("when something got a different lock recently", func() {
			It("still gets the lock", func() {
				lock, acquired, err := sqlDB.GetTaskLock(logger, "some-other-task-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				lock.Release()

				newLock, acquired, err := sqlDB.GetTaskLock(logger, "some-task-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				newLock.Release()
			})
		})
	})
})
