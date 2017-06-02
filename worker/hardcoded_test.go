package worker_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/worker"
)

var _ = Describe("Hardcoded", func() {
	var (
		logger           lager.Logger
		workerFactory    *dbfakes.FakeWorkerFactory
		gardenAddr       string
		baggageClaimAddr string
		resourceTypes    []atc.WorkerResourceType
		fakeClock        *fakeclock.FakeClock

		process ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("hardcoded-worker")
		workerFactory = new(dbfakes.FakeWorkerFactory)
		gardenAddr = "http://garden.example.com"
		baggageClaimAddr = "http://volumes.example.com"
		resourceTypes = []atc.WorkerResourceType{
			{
				Type:  "type",
				Image: "image",
			},
		}
		fakeClock = fakeclock.NewFakeClock(time.Now())
	})

	Describe("registering a single worker", func() {
		JustBeforeEach(func() {
			runner := worker.NewHardcoded(logger, workerFactory, fakeClock, gardenAddr, baggageClaimAddr, resourceTypes)
			process = ginkgomon.Invoke(runner)
		})

		AfterEach(func() {
			ginkgomon.Interrupt(process)
		})

		It("registers it and then keeps registering it on an interval", func() {
			expectedWorker := atc.Worker{
				Name:             gardenAddr,
				GardenAddr:       gardenAddr,
				BaggageclaimURL:  baggageClaimAddr,
				ActiveContainers: 0,
				ResourceTypes:    resourceTypes,
				Platform:         "linux",
				Tags:             []string{},
			}
			expectedTTL := 30 * time.Second

			Eventually(workerFactory.SaveWorkerCallCount()).Should(Equal(1))
			workerInfo, ttl := workerFactory.SaveWorkerArgsForCall(0)
			Expect(workerInfo).To(Equal(expectedWorker))
			Expect(ttl).To(Equal(expectedTTL))

			fakeClock.Increment(11 * time.Second)

			Eventually(workerFactory.SaveWorkerCallCount).Should(Equal(2))
			workerInfo, ttl = workerFactory.SaveWorkerArgsForCall(1)
			Expect(workerInfo).To(Equal(expectedWorker))
			Expect(ttl).To(Equal(expectedTTL))
		})

		It("can be interrupted", func() {
			expectedWorker := atc.Worker{
				Name:             gardenAddr,
				GardenAddr:       gardenAddr,
				BaggageclaimURL:  baggageClaimAddr,
				ActiveContainers: 0,
				ResourceTypes:    resourceTypes,
				Platform:         "linux",
				Tags:             []string{},
			}
			expectedTTL := 30 * time.Second

			Eventually(workerFactory.SaveWorkerCallCount()).Should(Equal(1))
			workerInfo, ttl := workerFactory.SaveWorkerArgsForCall(0)
			Expect(workerInfo).To(Equal(expectedWorker))
			Expect(ttl).To(Equal(expectedTTL))

			ginkgomon.Interrupt(process)

			fakeClock.Increment(11 * time.Second)

			Consistently(workerFactory.SaveWorkerCallCount).Should(Equal(1))
		})
	})

	Context("if saving to the DB fails", func() {
		disaster := errors.New("bad bad bad")

		BeforeEach(func() {
			workerFactory.SaveWorkerReturns(nil, disaster)
		})

		It("exits early", func() {
			runner := worker.NewHardcoded(logger, workerFactory, fakeClock, gardenAddr, baggageClaimAddr, resourceTypes)
			process = ifrit.Invoke(runner)

			Expect(<-process.Wait()).To(Equal(disaster))
		})
	})
})
