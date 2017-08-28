package db_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfigCheckSession", func() {
	var (
		ownerExpiries db.ContainerOwnerExpiries

		resourceConfigCheckSession db.ResourceConfigCheckSession
	)

	BeforeEach(func() {
		ownerExpiries = db.ContainerOwnerExpiries{
			GraceTime: 5 * time.Second,
			Min:       10 * time.Second,
			Max:       10 * time.Second,
		}
	})

	Describe("FindOrCreateResourceConfigCheckSession", func() {
		JustBeforeEach(func() {
			var err error
			resourceConfigCheckSession, err = resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(logger,
				defaultWorkerResourceType.Type,
				atc.Source{
					"some-type": "source",
				},
				creds.VersionedResourceTypes{},
				ownerExpiries,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when a resource config check session exists", func() {
			var existingRCCS db.ResourceConfigCheckSession

			BeforeEach(func() {
				var err error
				existingRCCS, err = resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(logger,
					defaultWorkerResourceType.Type,
					atc.Source{
						"some-type": "source",
					},
					creds.VersionedResourceTypes{},
					ownerExpiries,
				)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when we're not within the grace period", func() {
				It("finds the existing resource config check session", func() {
					Expect(resourceConfigCheckSession.ID()).To(Equal(existingRCCS.ID()))
				})
			})

			Context("when we're within the grace period of the existing check session", func() {
				BeforeEach(func() {
					time.Sleep(ownerExpiries.Max - ownerExpiries.GraceTime)
				})

				It("makes a new one", func() {
					Expect(resourceConfigCheckSession.ID()).ToNot(Equal(existingRCCS.ID()))
				})
			})
		})

		Context("when a different resource config exists", func() {
			var otherRCCS db.ResourceConfigCheckSession

			BeforeEach(func() {
				var err error
				otherRCCS, err = resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(logger,
					defaultWorkerResourceType.Type,
					atc.Source{
						"some-type": "different-source",
					},
					creds.VersionedResourceTypes{},
					ownerExpiries,
				)
				Expect(err).ToNot(HaveOccurred())
			})

			It("makes a new one", func() {
				Expect(resourceConfigCheckSession.ID()).ToNot(Equal(otherRCCS.ID()))
			})
		})
	})
})
