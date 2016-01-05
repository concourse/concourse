package worker_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/worker/fakes"
	"github.com/concourse/baggageclaim"
	bfakes "github.com/concourse/baggageclaim/fakes"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc/worker"
)

var _ = Describe("Volumes", func() {
	var (
		volumeFactory worker.VolumeFactory
		fakeVolume    *bfakes.FakeVolume
		fakeDB        *fakes.FakeVolumeFactoryDB
		fakeClock     *fakeclock.FakeClock
		logger        *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeVolume = new(bfakes.FakeVolume)
		fakeDB = new(fakes.FakeVolumeFactoryDB)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		logger = lagertest.NewTestLogger("test")

		volumeFactory = worker.NewVolumeFactory(fakeDB, fakeClock)
	})

	Context("VolumeFactory", func() {
		Describe("Build", func() {
			It("releases the volume it was given", func() {
				_, err := volumeFactory.Build(logger, fakeVolume)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeVolume.ReleaseCallCount()).To(Equal(1))
				actualTTL := fakeVolume.ReleaseArgsForCall(0)
				Expect(actualTTL).To(Equal(time.Duration(0)))
			})

			It("embeds the original volume in the wrapped volume", func() {
				fakeVolume.HandleReturns("some-handle")
				vol, err := volumeFactory.Build(logger, fakeVolume)
				Expect(err).ToNot(HaveOccurred())
				Expect(vol.Handle()).To(Equal("some-handle"))
			})
		})
	})

	Context("Volume", func() {
		var expectedTTL time.Duration
		var expectedTTL2 time.Duration

		BeforeEach(func() {
			expectedTTL = 10 * time.Second
			expectedTTL2 = 5 * time.Second
			fakeVolume.HandleReturns("some-handle")
			fakeDB.GetVolumeTTLReturns(expectedTTL, nil)
		})

		It("heartbeats", func() {
			vol, err := volumeFactory.Build(logger, fakeVolume)
			Expect(err).ToNot(HaveOccurred())

			By("looking up the initial ttl in the database")
			Expect(fakeDB.GetVolumeTTLCallCount()).To(Equal(1))
			actualHandle := fakeDB.GetVolumeTTLArgsForCall(0)
			Expect(actualHandle).To(Equal("some-handle"))

			By("using that ttl to heartbeat the volume initially")
			Expect(fakeVolume.SetTTLCallCount()).To(Equal(1))
			actualTTL := fakeVolume.SetTTLArgsForCall(0)
			Expect(actualTTL).To(Equal(expectedTTL))

			Expect(fakeDB.SetVolumeTTLCallCount()).To(Equal(1))
			actualHandle, actualTTL = fakeDB.SetVolumeTTLArgsForCall(0)
			Expect(actualHandle).To(Equal(vol.Handle()))
			Expect(actualTTL).To(Equal(expectedTTL))

			By("using the ttl from the database each tick")
			fakeDB.GetVolumeTTLReturns(expectedTTL2, nil)
			fakeClock.Increment(30 * time.Second)

			Eventually(fakeVolume.SetTTLCallCount).Should(Equal(2))
			actualTTL = fakeVolume.SetTTLArgsForCall(1)
			Expect(actualTTL).To(Equal(expectedTTL2))

			Eventually(fakeDB.SetVolumeTTLCallCount).Should(Equal(2))
			actualHandle, actualTTL = fakeDB.SetVolumeTTLArgsForCall(1)
			Expect(actualHandle).To(Equal(vol.Handle()))
			Expect(actualTTL).To(Equal(expectedTTL2))

			By("being resiliant to db errors")
			fakeDB.GetVolumeTTLReturns(0, errors.New("disaster"))
			fakeClock.Increment(30 * time.Second)
			Eventually(fakeVolume.SetTTLCallCount).Should(Equal(3))
			actualTTL = fakeVolume.SetTTLArgsForCall(2)
			Expect(actualTTL).To(Equal(expectedTTL2))

			By("releasing the volume with a final ttl")
			vol.Release(2 * time.Second)
			Eventually(fakeVolume.SetTTLCallCount).Should(Equal(4))
			actualTTL = fakeVolume.SetTTLArgsForCall(3)
			Expect(actualTTL).To(Equal(2 * time.Second))

			Eventually(fakeDB.SetVolumeTTLCallCount).Should(Equal(4))
			actualHandle, actualTTL = fakeDB.SetVolumeTTLArgsForCall(3)
			Expect(actualHandle).To(Equal(vol.Handle()))
			Expect(actualTTL).To(Equal(2 * time.Second))
		})

		It("is resiliant to errors while heartbeating", func() {
			By("using the baggage claim volumes ttl if the initial db lookup fails")
			fakeVolume.ExpirationReturns(expectedTTL, time.Now(), nil)
			fakeDB.GetVolumeTTLReturns(0, errors.New("disaster"))
			_, err := volumeFactory.Build(logger, fakeVolume)
			Expect(err).ToNot(HaveOccurred())
			By("using that ttl to heartbeat the volume initially")
			Expect(fakeVolume.SetTTLCallCount()).To(Equal(1))
			actualTTL := fakeVolume.SetTTLArgsForCall(0)
			Expect(actualTTL).To(Equal(expectedTTL))

			By("continuing to use the same ttl if the db continues to error")
			fakeClock.Increment(30 * time.Second)
			Eventually(fakeVolume.SetTTLCallCount).Should(Equal(2))
			actualTTL = fakeVolume.SetTTLArgsForCall(1)
			Expect(actualTTL).To(Equal(expectedTTL))
		})

		It("reaps the volume during heartbeat if the volume is not found", func() {
			fakeVolume.SetTTLReturns(baggageclaim.ErrVolumeNotFound)
			fakeVolume.HandleReturns("some-handle")

			_, err := volumeFactory.Build(logger, fakeVolume)
			Expect(err).ToNot(HaveOccurred())

			fakeClock.Increment(30 * time.Second)
			Expect(fakeDB.ReapVolumeCallCount()).To(Equal(1))
			Expect(fakeDB.ReapVolumeArgsForCall(0)).To(Equal("some-handle"))
		})
	})
})
