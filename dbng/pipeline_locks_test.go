package dbng_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PipelineLocks", func() {
	Describe("AcquireResourceCheckingLockWithIntervalCheck", func() {
		var someResource dbng.Resource

		BeforeEach(func() {
			var err error
			someResource, err = defaultPipeline.CreateResource("some-resource", atc.ResourceConfig{Type: "some-base-resource-type"})
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there has been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets the lock", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()

					lock, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})
			})

			Context("when not acquiring immediately", func() {
				It("does not get the lock", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()

					lock, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeFalse())
				})
			})
		})

		Context("when there has not been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets and keeps the lock and stops others from periodically getting it", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, false)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					lock, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})

				It("gets and keeps the lock and stops others from immediately getting it", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, true)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					lock, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})
			})

			Context("when not acquiring immediately", func() {
				It("gets and keeps the lock and stops others from periodically getting it", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, false)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					lock, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})

				It("gets and keeps the lock and stops others from immediately getting it", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, false)

					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, true)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					lock, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource, atc.VersionedResourceTypes{}, 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})
			})
		})
	})
})
