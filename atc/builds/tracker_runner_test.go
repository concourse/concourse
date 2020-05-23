package builds_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/builds"
	"github.com/concourse/concourse/atc/builds/buildsfakes"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
)

var _ = Describe("TrackerRunner", func() {
	var (
		trackerRunner ifrit.Runner
		process       ifrit.Process

		logger               *lagertest.TestLogger
		fakeTracker          *buildsfakes.FakeBuildTracker
		fakeComponent        *dbfakes.FakeComponent
		fakeLockFactory      *lockfakes.FakeLockFactory
		fakeComponentFactory *dbfakes.FakeComponentFactory
		fakeNotifications    *buildsfakes.FakeNotifications
		fakeClock            *fakeclock.FakeClock
		fakeLock             *lockfakes.FakeLock

		notifier   chan db.Notification
		trackTimes chan time.Time
		interval   = time.Minute
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		notifier = make(chan db.Notification, 1)
		fakeTracker = new(buildsfakes.FakeBuildTracker)
		fakeNotifications = new(buildsfakes.FakeNotifications)
		fakeNotifications.ListenReturns(notifier, nil)
		fakeComponent = new(dbfakes.FakeComponent)
		fakeComponentFactory = new(dbfakes.FakeComponentFactory)
		fakeComponentFactory.FindReturns(fakeComponent, true, nil)
		fakeLockFactory = new(lockfakes.FakeLockFactory)
		fakeLock = new(lockfakes.FakeLock)
		fakeClock = fakeclock.NewFakeClock(time.Unix(0, 123))

		trackTimes = make(chan time.Time, 1)
		fakeTracker.TrackStub = func() error {
			trackTimes <- fakeClock.Now()
			return nil
		}

		trackerRunner = builds.NewRunner(
			logger,
			fakeClock,
			fakeTracker,
			interval,
			fakeNotifications,
			fakeLockFactory,
			fakeComponentFactory,
		)
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(trackerRunner)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	Context("when the interval elapses", func() {
		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(interval)
		})

		It("calls to get a lock for cache invalidation", func() {
			Eventually(fakeLockFactory.AcquireCallCount).Should(Equal(1))
			_, lockID := fakeLockFactory.AcquireArgsForCall(0)
			Expect(lockID).To(Equal(lock.NewTaskLockID(atc.ComponentBuildTracker)))
		})

		Context("when getting a lock succeeds", func() {
			BeforeEach(func() {
				fakeLockFactory.AcquireReturns(fakeLock, true, nil)
			})

			Context("when getting the component fails", func() {
				BeforeEach(func() {
					fakeComponentFactory.FindReturns(nil, false, errors.New("nope"))
				})

				It("does not exit and does not run the task", func() {
					Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
					Consistently(process.Wait()).ShouldNot(Receive())
				})
			})

			Context("when the component is not found", func() {
				BeforeEach(func() {
					fakeComponentFactory.FindReturns(nil, false, nil)
				})

				It("does not exit and does not run the task", func() {
					Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
					Consistently(process.Wait()).ShouldNot(Receive())
				})
			})

			Context("when getting the component succeeds", func() {
				BeforeEach(func() {
					fakeComponentFactory.FindReturns(fakeComponent, true, nil)
				})

				Context("when the component is paused", func() {
					BeforeEach(func() {
						fakeComponent.PausedReturns(true)
					})

					It("does not exit and does not run the task", func() {
						Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
						Consistently(process.Wait()).ShouldNot(Receive())
					})
				})

				Context("when the component is unpaused", func() {
					BeforeEach(func() {
						fakeComponent.PausedReturns(false)
					})

					Context("when the interval has not elapsed", func() {
						BeforeEach(func() {
							fakeComponent.IntervalElapsedReturns(false)
						})

						It("does not exit and does not run the task", func() {
							Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
							Consistently(process.Wait()).ShouldNot(Receive())
						})
					})

					Context("when the interval has elapsed", func() {
						BeforeEach(func() {
							fakeComponent.IntervalElapsedReturns(true)
						})

						It("it runs the task", func() {
							Eventually(fakeTracker.TrackCallCount).Should(Equal(1))
						})

						It("updates last ran", func() {
							Eventually(fakeComponent.UpdateLastRanCallCount).Should(Equal(1))
						})

						It("releases the lock", func() {
							Eventually(fakeLock.ReleaseCallCount).Should(Equal(1))
						})

						Context("when running the task fails", func() {
							BeforeEach(func() {
								fakeTracker.TrackReturns(errors.New("disaster"))
							})

							It("does not exit the process", func() {
								Consistently(process.Wait()).ShouldNot(Receive())
							})

							It("does not update last ran", func() {
								Consistently(fakeComponent.UpdateLastRanCallCount).Should(Equal(0))
							})

							It("releases the lock", func() {
								Eventually(fakeLock.ReleaseCallCount).Should(Equal(1))
							})
						})
					})
				})
			})
		})

		Context("when getting a lock fails", func() {
			Context("because of an error", func() {
				BeforeEach(func() {
					fakeLockFactory.AcquireReturns(nil, true, errors.New("disaster"))
				})

				It("does not exit and does not run the task", func() {
					Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
					Consistently(process.Wait()).ShouldNot(Receive())
				})

				It("does not update last ran", func() {
					Consistently(fakeComponent.UpdateLastRanCallCount).Should(Equal(0))
				})
			})

			Context("because we got acquired of false", func() {
				BeforeEach(func() {
					fakeLockFactory.AcquireReturns(nil, false, nil)
				})

				It("does not exit and does not run the task", func() {
					Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
					Consistently(process.Wait()).ShouldNot(Receive())
				})

				It("does not update last ran", func() {
					Consistently(fakeComponent.UpdateLastRanCallCount).Should(Equal(0))
				})
			})
		})
	})

	Context("when it receives a notification", func() {
		BeforeEach(func() {
			notifier <- db.Notification{Healthy: true}
		})

		It("calls to get a lock for cache invalidation", func() {
			Eventually(fakeLockFactory.AcquireCallCount).Should(Equal(1))
			_, lockID := fakeLockFactory.AcquireArgsForCall(0)
			Expect(lockID).To(Equal(lock.NewTaskLockID(atc.ComponentBuildTracker)))
		})

		Context("when getting a lock succeeds", func() {
			BeforeEach(func() {
				fakeLockFactory.AcquireReturns(fakeLock, true, nil)
			})

			Context("when getting the component fails", func() {
				BeforeEach(func() {
					fakeComponentFactory.FindReturns(nil, false, errors.New("nope"))
				})

				It("does not exit and does not run the task", func() {
					Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
					Consistently(process.Wait()).ShouldNot(Receive())
				})
			})

			Context("when the component is not found", func() {
				BeforeEach(func() {
					fakeComponentFactory.FindReturns(nil, false, nil)
				})

				It("does not exit and does not run the task", func() {
					Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
					Consistently(process.Wait()).ShouldNot(Receive())
				})
			})

			Context("when getting the component succeeds", func() {
				BeforeEach(func() {
					fakeComponentFactory.FindReturns(fakeComponent, true, nil)
				})

				Context("when the component is paused", func() {
					BeforeEach(func() {
						fakeComponent.PausedReturns(true)
					})

					It("does not exit and does not run the task", func() {
						Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
						Consistently(process.Wait()).ShouldNot(Receive())
					})
				})

				Context("when the component is unpaused", func() {
					BeforeEach(func() {
						fakeComponent.PausedReturns(false)
					})

					Context("when the interval has not elapsed", func() {
						BeforeEach(func() {
							fakeComponent.IntervalElapsedReturns(false)
						})

						It("still runs the task", func() {
							Eventually(fakeTracker.TrackCallCount).Should(Equal(1))
							Consistently(process.Wait()).ShouldNot(Receive())
						})
					})

					Context("when the interval has elapsed", func() {
						BeforeEach(func() {
							fakeComponent.IntervalElapsedReturns(true)
						})

						It("it runs the task", func() {
							Eventually(fakeTracker.TrackCallCount).Should(Equal(1))
						})

						It("updates last ran", func() {
							Eventually(fakeComponent.UpdateLastRanCallCount).Should(Equal(1))
						})

						It("releases the lock", func() {
							Eventually(fakeLock.ReleaseCallCount).Should(Equal(1))
						})

						Context("when running the task fails", func() {
							BeforeEach(func() {
								fakeTracker.TrackReturns(errors.New("disaster"))
							})

							It("does not exit the process", func() {
								Consistently(process.Wait()).ShouldNot(Receive())
							})

							It("does not update last ran", func() {
								Consistently(fakeComponent.UpdateLastRanCallCount).Should(Equal(0))
							})

							It("releases the lock", func() {
								Eventually(fakeLock.ReleaseCallCount).Should(Equal(1))
							})
						})
					})
				})
			})
		})

		Context("when getting a lock fails", func() {
			Context("because of an error", func() {
				BeforeEach(func() {
					fakeLockFactory.AcquireReturns(nil, true, errors.New("disaster"))
				})

				It("does not exit and does not run the task", func() {
					Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
					Consistently(process.Wait()).ShouldNot(Receive())
				})

				It("does not update last ran", func() {
					Consistently(fakeComponent.UpdateLastRanCallCount).Should(Equal(0))
				})
			})

			Context("because we got acquired of false", func() {
				BeforeEach(func() {
					fakeLockFactory.AcquireReturns(nil, false, nil)
				})

				It("does not exit and does not run the task", func() {
					Consistently(fakeTracker.TrackCallCount).Should(Equal(0))
					Consistently(process.Wait()).ShouldNot(Receive())
				})

				It("does not update last ran", func() {
					Consistently(fakeComponent.UpdateLastRanCallCount).Should(Equal(0))
				})
			})
		})
	})

	Context("when it receives shutdown signal", func() {
		JustBeforeEach(func() {
			go func() {
				process.Signal(os.Interrupt)
			}()
		})

		It("releases tracker", func() {
			Eventually(fakeTracker.ReleaseCallCount).Should(Equal(1))
		})

		It("notifies other atc it is shutting down", func() {
			Eventually(fakeNotifications.NotifyCallCount).Should(Equal(1))
		})
	})
})
