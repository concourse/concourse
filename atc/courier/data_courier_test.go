package courier_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/courier"
	"github.com/concourse/concourse/atc/courier/courierfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Data Courier", func() {
	var fakeMigrator *courierfakes.FakeMigrator
	var fakeLock *lockfakes.FakeLock
	var fakeRunner *courierfakes.FakeRunner
	var result chan ifrit.Process
	var acquired chan bool

	BeforeEach(func() {
		acquired = make(chan bool, 1)

		fakeMigrator = new(courierfakes.FakeMigrator)
		fakeMigrator.AcquireMigrationLockStub = func(logger lager.Logger) (lock.Lock, bool, error) {
			select {
			case <-acquired:
				return fakeLock, true, nil
			case <-time.After(time.Second):
				return nil, false, nil
			}
		}

		fakeRunner = new(courierfakes.FakeRunner)
		fakeLock = new(lockfakes.FakeLock)
	})

	JustBeforeEach(func() {
		result = make(chan ifrit.Process)
		go func() {
			result <- ifrit.Invoke(courier.NewDataCourier(
				nil,
				fakeRunner,
				fakeMigrator,
			))
		}()
	})

	AfterEach(func() {
		process := <-result
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	Context("when the lock is acquired", func() {
		It("does not migrate the data", func() {
			Consistently(fakeMigrator.MigrateCallCount).Should(Equal(0))
			Consistently(fakeRunner.RunCallCount).Should(Equal(0))

			By("acquiring the lock, we are able to migrate the data")
			acquired <- true

			Eventually(fakeMigrator.MigrateCallCount).Should(Equal(1))
			Eventually(fakeRunner.RunCallCount).Should(Equal(1))
		})
	})
})
