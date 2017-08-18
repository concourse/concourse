package db_test

import (
	"time"

	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PipelineLocks", func() {
	Describe("AcquireResourceCheckingLockWithIntervalCheck", func() {
		var (
			someResource               db.Resource
			resourceConfigCheckSession db.ResourceConfigCheckSession
			variables                  creds.Variables
		)

		ownerExpiries := db.ContainerOwnerExpiries{
			GraceTime: 1 * time.Minute,
			Min:       5 * time.Minute,
			Max:       5 * time.Minute,
		}

		BeforeEach(func() {
			variables = template.StaticVariables{}

			var err error
			var found bool

			someResource, found, err = defaultPipeline.Resource("some-resource")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			pipelineResourceTypes, err := defaultPipeline.ResourceTypes()
			Expect(err).NotTo(HaveOccurred())

			resourceConfigCheckSession, err = resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(
				logger,
				someResource.Type(),
				someResource.Source(),
				creds.NewVersionedResourceTypes(template.StaticVariables{}, pipelineResourceTypes.Deserialize()),
				ownerExpiries,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there has been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets the lock", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()

					lock, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})
			})

			Context("when not acquiring immediately", func() {
				It("does not get the lock", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()

					lock, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeFalse())
				})
			})
		})

		Context("when there has not been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets and keeps the lock and stops others from periodically getting it", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					lock, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})

				It("gets and keeps the lock and stops others from immediately getting it", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, true)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					lock, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})
			})

			Context("when not acquiring immediately", func() {
				It("gets and keeps the lock and stops others from periodically getting it", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					lock, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})

				It("gets and keeps the lock and stops others from immediately getting it", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)

					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, true)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					lock, acquired, err = defaultPipeline.AcquireResourceCheckingLockWithIntervalCheck(logger, someResource.Name(), resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					lock.Release()
				})
			})
		})
	})

	Describe("AcquireResourceTypeCheckingLockWithIntervalCheck", func() {
		var (
			resourceConfigCheckSession db.ResourceConfigCheckSession
			variables                  creds.Variables
		)

		ownerExpiries := db.ContainerOwnerExpiries{
			GraceTime: 1 * time.Minute,
			Min:       5 * time.Minute,
			Max:       5 * time.Minute,
		}

		BeforeEach(func() {
			variables = template.StaticVariables{}

			someResourceType, found, err := defaultPipeline.ResourceType("some-type")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			pipelineResourceTypes, err := defaultPipeline.ResourceTypes()
			Expect(err).NotTo(HaveOccurred())

			resourceConfigCheckSession, err = resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(
				logger,
				someResourceType.Type(),
				someResourceType.Source(),
				creds.NewVersionedResourceTypes(template.StaticVariables{}, pipelineResourceTypes.Deserialize().Without(someResourceType.Name())),
				ownerExpiries,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there has been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets the lock", func() {
					dbLock, acquired, err := defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					dbLock.Release()

					dbLock, acquired, err = defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					dbLock.Release()
				})
			})

			Context("when not acquiring immediately", func() {
				It("does not get the lock", func() {
					dbLock, acquired, err := defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					dbLock.Release()

					_, acquired, err = defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeFalse())

					dbLock.Release()
				})
			})
		})

		Context("when there has not been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets and keeps the lock and stops others from periodically getting it", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					newLock, acquired, err := defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					newLock.Release()
				})

				It("gets and keeps the lock and stops others from immediately getting it", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, true)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					newLock, acquired, err := defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					newLock.Release()
				})
			})

			Context("when not acquiring immediately", func() {
				It("gets and keeps the lock and stops others from periodically getting it", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					newLock, acquired, err := defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					newLock.Release()
				})

				It("gets and keeps the lock and stops others from immediately getting it", func() {
					lock, acquired, err := defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, true)
						Expect(err).NotTo(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					lock.Release()

					time.Sleep(time.Second)

					newLock, acquired, err := defaultPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, "some-type", resourceConfigCheckSession.ResourceConfig(), 1*time.Second, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(acquired).To(BeTrue())

					newLock.Release()
				})
			})
		})
	})
})
