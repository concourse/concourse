package worker_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/fakes"
)

var _ = Describe("SingleWorker", func() {
	Describe("registering a single worker", func() {
		It("registers it and then keeps registering iton an interval", func() {
			logger := lagertest.NewTestLogger("single-worker")
			workerDB := &fakes.FakeSaveWorkerDB{}
			gardenAddr := "http://garden.example.com"
			baggageClaimAddr := "http://volumes.example.com"
			resourceTypes := []atc.WorkerResourceType{
				{
					Type:  "type",
					Image: "image",
				},
			}

			fakeClock := fakeclock.NewFakeClock(time.Now())

			worker.RegisterSingleWorker(logger, workerDB, fakeClock, gardenAddr, baggageClaimAddr, resourceTypes)

			expectedWorkerInfo := db.WorkerInfo{
				Name:             gardenAddr,
				GardenAddr:       gardenAddr,
				BaggageclaimURL:  baggageClaimAddr,
				ActiveContainers: 0,
				ResourceTypes:    resourceTypes,
				Platform:         "linux",
				Tags:             []string{},
			}

			Expect(workerDB.SaveWorkerCallCount()).To(Equal(1))
			workerInfo, ttl := workerDB.SaveWorkerArgsForCall(0)
			Expect(workerInfo).To(Equal(expectedWorkerInfo))
			Expect(ttl).To(Equal(30 * time.Second))

			fakeClock.Increment(11 * time.Second)

			Eventually(workerDB.SaveWorkerCallCount).Should(Equal(2))

			workerInfo, ttl = workerDB.SaveWorkerArgsForCall(1)
			Expect(workerInfo).To(Equal(expectedWorkerInfo))
			Expect(ttl).To(Equal(30 * time.Second))
		})
	})
})
