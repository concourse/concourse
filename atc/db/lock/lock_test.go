package lock_test

import (
	"database/sql"
	"sync"
	"time"

	"code.cloudfoundry.org/lager/v3"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Locks", func() {
	var (
		lockFactory lock.LockFactory

		dbLock lock.Lock

		dbConn db.DbConn

		team        db.Team
		teamFactory db.TeamFactory

		logger      *lagertest.TestLogger
		fakeLogFunc = func(logger lager.Logger, id lock.LockID) {}

		lockConns [lock.FactoryCount]*sql.DB
	)

	BeforeEach(func() {
		postgresRunner.CreateTestDBFromTemplate()

		logger = lagertest.NewTestLogger("test")

		for i := 0; i < lock.FactoryCount; i++ {
			lockConns[i] = postgresRunner.OpenSingleton()
		}

		lockFactory = lock.NewLockFactory(lockConns, fakeLogFunc, fakeLogFunc)

		dbConn = postgresRunner.OpenConn()
		teamFactory = db.NewTeamFactory(dbConn, lockFactory)

		var err error
		team, err = teamFactory.CreateTeam(atc.Team{Name: "team-name"})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		if dbLock != nil {
			_ = dbLock.Release()
		}

		postgresRunner.DropTestDB()
	})

	Describe("locks in general", func() {
		It("Acquire can only obtain lock once", func() {
			var acquired bool
			var err error
			dbLock, acquired, err = lockFactory.Acquire(logger, lock.LockID{42})
			Expect(err).NotTo(HaveOccurred())
			Expect(acquired).To(BeTrue())

			_, acquired, err = lockFactory.Acquire(logger, lock.LockID{42})
			Expect(err).NotTo(HaveOccurred())
			Expect(acquired).To(BeFalse())
		})

		It("Acquire accepts list of ids", func() {
			var acquired bool
			var err error
			dbLock, acquired, err = lockFactory.Acquire(logger, lock.LockID{42, 56})
			Expect(err).NotTo(HaveOccurred())
			Expect(acquired).To(BeTrue())

			Consistently(func() error {
				connCount := 3

				var anyError error
				var wg sync.WaitGroup
				wg.Add(connCount)

				for i := 0; i < connCount; i++ {
					go func() {
						defer wg.Done()

						_, _, err := lockFactory.Acquire(logger, lock.LockID{42, 56})
						if err != nil {
							anyError = err
						}

					}()
				}

				wg.Wait()

				return anyError
			}, 1500*time.Millisecond, 100*time.Millisecond).ShouldNot(HaveOccurred())

			dbLock, acquired, err = lockFactory.Acquire(logger, lock.LockID{56, 42})
			Expect(err).NotTo(HaveOccurred())
			Expect(acquired).To(BeTrue())

			_, acquired, err = lockFactory.Acquire(logger, lock.LockID{56, 42})
			Expect(err).NotTo(HaveOccurred())
			Expect(acquired).To(BeFalse())
		})

		Context("when another connection is holding the lock", func() {
			var lockFactory2 lock.LockFactory
			var lockConns2 [lock.FactoryCount]*sql.DB

			BeforeEach(func() {
				for i := 0; i < lock.FactoryCount; i++ {
					lockConns2[i] = postgresRunner.OpenSingleton()
				}
				lockFactory2 = lock.NewLockFactory(lockConns2, fakeLogFunc, fakeLogFunc)
			})

			It("does not acquire the lock", func() {
				var acquired bool
				var err error
				dbLock, acquired, err = lockFactory.Acquire(logger, lock.LockID{42})
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				_, acquired, err = lockFactory2.Acquire(logger, lock.LockID{42})
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeFalse())

				err = dbLock.Release()
				Expect(err).NotTo(HaveOccurred())
			})

			It("acquires the locks once it is released", func() {
				var acquired bool
				var err error
				dbLock, acquired, err = lockFactory.Acquire(logger, lock.LockID{42})
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				_, acquired, err = lockFactory2.Acquire(logger, lock.LockID{42})
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeFalse())

				err = dbLock.Release()
				Expect(err).NotTo(HaveOccurred())

				dbLock2, acquired, err := lockFactory2.Acquire(logger, lock.LockID{42})
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				err = dbLock2.Release()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when two locks are being acquired at the same time", func() {
			var fakeLockDB *lockfakes.FakeLockDB
			var acquiredLock2 chan struct{}
			var lock2Err error
			var lock2Acquired bool
			var fakeLockFactory lock.LockFactory

			BeforeEach(func() {
				fakeLockDB = new(lockfakes.FakeLockDB)
				fakeLockFactory = lock.NewTestLockFactory(fakeLockDB)
				acquiredLock2 = make(chan struct{})

				called := false
				readyToAcquire := make(chan struct{})

				fakeLockDB.AcquireStub = func(id lock.LockID) (bool, error) {
					if !called {
						called = true

						go func() {
							close(readyToAcquire)
							_, lock2Acquired, lock2Err = fakeLockFactory.Acquire(logger, id)
							close(acquiredLock2)
						}()

						<-readyToAcquire
					}

					return true, nil
				}
			})

			It("only acquires one of the locks", func() {
				_, acquired, err := fakeLockFactory.Acquire(logger, lock.LockID{57})
				Expect(err).NotTo(HaveOccurred())
				Expect(acquired).To(BeTrue())

				<-acquiredLock2

				Expect(lock2Err).NotTo(HaveOccurred())
				Expect(lock2Acquired).To(BeFalse())
			})

			Context("when locks are being created on different lock factory (different db conn)", func() {
				var fakeLockFactory2 lock.LockFactory

				BeforeEach(func() {
					fakeLockFactory2 = lock.NewTestLockFactory(fakeLockDB)

					called := false
					readyToAcquire := make(chan struct{})

					fakeLockDB.AcquireStub = func(id lock.LockID) (bool, error) {
						if !called {
							called = true

							go func() {
								close(readyToAcquire)
								_, lock2Acquired, lock2Err = fakeLockFactory2.Acquire(logger, id)
								close(acquiredLock2)
							}()

							<-readyToAcquire
						}

						return true, nil
					}
				})

				It("allows to acquire both locks", func() {
					_, acquired, err := fakeLockFactory.Acquire(logger, lock.LockID{57})
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					<-acquiredLock2

					Expect(lock2Err).NotTo(HaveOccurred())
					Expect(lock2Acquired).To(BeTrue())
				})
			})
		})
	})

	Describe("taking out a lock on build tracking", func() {
		var build db.Build

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

			err = lock.Release()
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(time.Second)

			newLock, acquired, err := build.AcquireTrackingLock(logger, 1*time.Second)
			Expect(err).NotTo(HaveOccurred())
			Expect(acquired).To(BeTrue())

			err = newLock.Release()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
