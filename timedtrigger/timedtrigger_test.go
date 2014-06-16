package timedtrigger_test

import (
	"os"
	"time"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"

	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/queue/fakequeuer"
	. "github.com/winston-ci/winston/timedtrigger"
)

var _ = Describe("TimedTrigger", func() {
	var (
		jobs   config.Jobs
		queuer *fakequeuer.FakeQueuer

		process ifrit.Process
	)

	jobSecond := config.Job{
		Name:         "one-second",
		TriggerEvery: config.Duration(1 * time.Second),
	}

	jobHalfSecond := config.Job{
		Name:         "half-second",
		TriggerEvery: config.Duration(500 * time.Millisecond),
	}

	jobNone := config.Job{Name: "no-trigger"}

	BeforeEach(func() {
		jobs = config.Jobs{
			jobSecond,
			jobHalfSecond,
			jobNone,
		}

		queuer = new(fakequeuer.FakeQueuer)
	})

	JustBeforeEach(func() {
		process = ifrit.Envoke(NewTimer(jobs, queuer))
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	It("triggers jobs on their configured interval", func() {
		start := time.Now()

		Eventually(queuer.TriggerCallCount, 1*time.Second).Should(BeNumerically(">=", 1))
		t1 := time.Now()

		Eventually(queuer.TriggerCallCount, 2*time.Second).Should(BeNumerically(">=", 2))
		t2 := time.Now()

		Ω(t1.Sub(start)).Should(BeNumerically("~", 500*time.Millisecond, 100*time.Millisecond))
		Ω(t2.Sub(start)).Should(BeNumerically("~", 1*time.Second, 100*time.Millisecond))

		Ω(queuer.TriggerArgsForCall(0)).Should(Equal(jobHalfSecond))
		Ω(queuer.TriggerArgsForCall(1)).Should(Equal(jobSecond))
	})
})
