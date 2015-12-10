package lostandfound_test

import (
	"errors"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/lostandfound"
	"github.com/concourse/atc/lostandfound/fakes"
	"github.com/pivotal-golang/lager/lagertest"

	dbfakes "github.com/concourse/atc/db/fakes"
	wfakes "github.com/concourse/atc/worker/fakes"
	bcfakes "github.com/concourse/baggageclaim/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volumes are reaped", func() {
	var (
		fakeWorkerClient       *wfakes.FakeClient
		fakeWorker             *wfakes.FakeWorker
		fakeBaggageClaimClient *bcfakes.FakeClient

		fakePipelineDBFactory          *dbfakes.FakePipelineDBFactory
		fakeBaggageCollectorDB         *fakes.FakeBaggageCollectorDB
		expectedOldResourceGracePeriod = 4 * time.Minute

		baggageCollector    lostandfound.BaggageCollector
		returnedSavedVolume db.SavedVolume
	)

	BeforeEach(func() {
		fakeWorkerClient = new(wfakes.FakeClient)
		fakeWorker = new(wfakes.FakeWorker)
		fakeBaggageClaimClient = new(bcfakes.FakeClient)
		baggageCollectorLogger := lagertest.NewTestLogger("test")
		fakeBaggageCollectorDB = new(fakes.FakeBaggageCollectorDB)
		fakePipelineDBFactory = new(dbfakes.FakePipelineDBFactory)

		baggageCollector = lostandfound.NewBaggageCollector(
			baggageCollectorLogger,
			fakeWorkerClient,
			fakeBaggageCollectorDB,
			fakePipelineDBFactory,
			expectedOldResourceGracePeriod,
		)

		returnedSavedVolume = db.SavedVolume{
			Volume: db.Volume{
				WorkerName:      "test-worker",
				TTL:             time.Minute,
				Handle:          "some-handle",
				ResourceVersion: atc.Version{"some": "version"},
				ResourceHash:    "some-hash",
			},
			ID:        123,
			ExpiresIn: expectedOldResourceGracePeriod,
		}

		fakeBaggageCollectorDB.GetVolumesReturns([]db.SavedVolume{returnedSavedVolume}, nil)
	})

	Context("when volume no longer exists", func() {
		Context("when the worker is no longer around", func() {
			BeforeEach(func() {
				fakeWorkerClient.GetWorkerReturns(nil, errors.New("no-worker-found"))
			})

			It("removes the volume from the database", func() {
				err := baggageCollector.Collect()
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBaggageCollectorDB.ReapVolumeCallCount()).To(Equal(1))
				Expect(fakeBaggageCollectorDB.ReapVolumeArgsForCall(0)).To(Equal(returnedSavedVolume.Handle))
			})
		})

		Context("baggage claim is no longer found on the worker", func() {
			BeforeEach(func() {
				fakeWorkerClient.GetWorkerReturns(fakeWorker, nil)
				fakeWorker.VolumeManagerReturns(nil, false)
			})

			It("removes the volume from the database", func() {
				err := baggageCollector.Collect()
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBaggageCollectorDB.ReapVolumeCallCount()).To(Equal(1))
				Expect(fakeBaggageCollectorDB.ReapVolumeArgsForCall(0)).To(Equal(returnedSavedVolume.Handle))
			})
		})

		Context("the volume is no longer found on the worker", func() {
			BeforeEach(func() {
				fakeWorkerClient.GetWorkerReturns(fakeWorker, nil)
				fakeWorker.VolumeManagerReturns(nil, false)
				fakeBaggageClaimClient.LookupVolumeReturns(nil, false, errors.New("could-not-locate-volume"))
			})

			It("removes the volume from the database", func() {
				err := baggageCollector.Collect()
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBaggageCollectorDB.ReapVolumeCallCount()).To(Equal(1))
				Expect(fakeBaggageCollectorDB.ReapVolumeArgsForCall(0)).To(Equal(returnedSavedVolume.Handle))
			})
		})

	})

})
