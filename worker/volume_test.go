package worker_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"
	bfakes "github.com/concourse/baggageclaim/baggageclaimfakes"
)

var _ = Describe("Volumes", func() {
	var (
		volumeFactory worker.VolumeFactory
		fakeVolume    *bfakes.FakeVolume
		fakeDB        *workerfakes.FakeVolumeFactoryDB
		fakeClock     *fakeclock.FakeClock
		logger        *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeVolume = new(bfakes.FakeVolume)
		fakeVolume.HandleReturns("some-handle")

		fakeDB = new(workerfakes.FakeVolumeFactoryDB)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		logger = lagertest.NewTestLogger("test")

		volumeFactory = worker.NewVolumeFactory(fakeDB, fakeClock)
	})

	Context("VolumeFactory", func() {
		Describe("BuildWithIndefiniteTTL", func() {
			Context("when the volume's TTL can be found", func() {
				BeforeEach(func() {
					fakeDB.GetVolumeTTLReturns(time.Minute, true, nil)
				})

				It("releases the volume it was given", func() {
					_, err := volumeFactory.BuildWithIndefiniteTTL(logger, fakeVolume)
					Expect(err).ToNot(HaveOccurred())
					Expect(fakeVolume.ReleaseCallCount()).To(Equal(1))
					actualTTL := fakeVolume.ReleaseArgsForCall(0)
					Expect(actualTTL).To(BeNil())
				})

				It("embeds the original volume in the wrapped volume", func() {
					vol, err := volumeFactory.BuildWithIndefiniteTTL(logger, fakeVolume)
					Expect(err).ToNot(HaveOccurred())
					Expect(vol.Handle()).To(Equal("some-handle"))
				})
			})

			Context("when the volume's TTL cannot be found", func() {
				BeforeEach(func() {
					fakeDB.GetVolumeTTLReturns(0, false, nil)
				})

				It("releases the volume it was given and returns false", func() {
					_, err := volumeFactory.BuildWithIndefiniteTTL(logger, fakeVolume)
					Expect(err).ToNot(HaveOccurred())
					Expect(fakeVolume.ReleaseCallCount()).To(Equal(1))
					actualTTL := fakeVolume.ReleaseArgsForCall(0)
					Expect(actualTTL).To(BeNil())
				})
			})
		})
	})
})
