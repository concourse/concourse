package lockrunner_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
	. "github.com/concourse/atc/lockrunner"
	"github.com/concourse/atc/lockrunner/lockrunnerfakes"
)

var _ = Describe("Runner", func() {
	var (
		fakeLockFactory *lockfakes.FakeLockFactory
		fakeTask        *lockrunnerfakes.FakeTask
		fakeClock       *fakeclock.FakeClock
		fakeLock        *lockfakes.FakeLock

		interval time.Duration

		process ifrit.Process
	)

	BeforeEach(func() {
		fakeLockFactory = new(lockfakes.FakeLockFactory)
		fakeTask = new(lockrunnerfakes.FakeTask)
		fakeLock = new(lockfakes.FakeLock)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))

		interval = 100 * time.Millisecond
	})

	JustBeforeEach(func() {
		process = ginkgomon.Invoke(NewRunner(
			lagertest.NewTestLogger("test"),
			fakeTask,
			"some-task-name",
			fakeLockFactory,
			fakeClock,
			interval,
		))
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Expect(<-process.Wait()).ToNot(HaveOccurred())
	})

	Context("when the interval elapses", func() {
		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(interval)
		})

		It("calls to get a lock for cache invalidation", func() {
			Eventually(fakeLockFactory.AcquireCallCount).Should(Equal(1))
			_, lockID := fakeLockFactory.AcquireArgsForCall(0)
			Expect(lockID).To(Equal(lock.NewTaskLockID("some-task-name")))
		})

		Context("when getting a lock succeeds", func() {
			BeforeEach(func() {
				fakeLockFactory.AcquireReturns(fakeLock, true, nil)
			})

			It("it collects lost baggage", func() {
				Eventually(fakeTask.RunCallCount).Should(Equal(1))
			})

			It("releases the lock", func() {
				Eventually(fakeLock.ReleaseCallCount).Should(Equal(1))
			})

			Context("when collecting fails", func() {
				BeforeEach(func() {
					fakeTask.RunReturns(errors.New("disaster"))
				})

				It("does not exit the process", func() {
					Consistently(process.Wait()).ShouldNot(Receive())
				})

				It("releases the lock", func() {
					Eventually(fakeLock.ReleaseCallCount).Should(Equal(1))
				})
			})
		})

		Context("when getting a lock fails", func() {
			Context("because of an error", func() {
				BeforeEach(func() {
					fakeLockFactory.AcquireReturns(nil, true, errors.New("disaster"))
				})

				It("does not exit and does not collect baggage", func() {
					Consistently(fakeTask.RunCallCount).Should(Equal(0))
					Consistently(process.Wait()).ShouldNot(Receive())
				})
			})

			Context("because we got acquired of false", func() {
				BeforeEach(func() {
					fakeLockFactory.AcquireReturns(nil, false, nil)
				})

				It("does not exit and does not collect baggage", func() {
					Consistently(fakeTask.RunCallCount).Should(Equal(0))
					Consistently(process.Wait()).ShouldNot(Receive())
				})
			})
		})
	})
})
