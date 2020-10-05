package db_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfig", func() {
	var resourceConfig db.ResourceConfig

	Context("when non-unique", func() {
		BeforeEach(func() {
			types, err := defaultPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())

			resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(
				defaultWorkerResourceType.Type,
				atc.Source{"some": "source"},
				types.Deserialize(),
			)
			Expect(err).ToNot(HaveOccurred())
		})

		Describe("FindOrCreateScope", func() {
			Context("given no resource", func() {
				It("finds or creates a global scope", func() {
					createdScope, err := resourceConfig.FindOrCreateScope(nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(createdScope.Resource()).To(BeNil())
					Expect(createdScope.ResourceConfig().ID()).To(Equal(resourceConfig.ID()))

					foundScope, err := resourceConfig.FindOrCreateScope(nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(foundScope.ID()).To(Equal(createdScope.ID()))
				})
			})

			Context("given a resource", func() {
				Context("with global resources disabled", func() {
					BeforeEach(func() {
						// XXX(check-refactor): make this non global
						atc.EnableGlobalResources = false
					})

					It("finds or creates a unique scope", func() {
						createdScope, err := resourceConfig.FindOrCreateScope(defaultResource)
						Expect(err).ToNot(HaveOccurred())
						Expect(createdScope.Resource()).ToNot(BeNil())
						Expect(createdScope.Resource().ID()).To(Equal(defaultResource.ID()))
						Expect(createdScope.ResourceConfig().ID()).To(Equal(resourceConfig.ID()))

						foundScope, err := resourceConfig.FindOrCreateScope(defaultResource)
						Expect(err).ToNot(HaveOccurred())
						Expect(foundScope.ID()).To(Equal(createdScope.ID()))
					})
				})

				Context("with global resources enabled", func() {
					BeforeEach(func() {
						atc.EnableGlobalResources = true
					})

					It("finds or creates a global scope", func() {
						createdScope, err := resourceConfig.FindOrCreateScope(defaultResource)
						Expect(err).ToNot(HaveOccurred())
						Expect(createdScope.Resource()).To(BeNil())
						Expect(createdScope.ResourceConfig().ID()).To(Equal(resourceConfig.ID()))

						foundScope, err := resourceConfig.FindOrCreateScope(defaultResource)
						Expect(err).ToNot(HaveOccurred())
						Expect(foundScope.ID()).To(Equal(createdScope.ID()))
					})
				})
			})
		})
	})

	Context("when using a unique base resource type", func() {
		BeforeEach(func() {
			types, err := defaultPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())

			resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(
				uniqueWorkerResourceType.Type,
				atc.Source{"some": "source"},
				types.Deserialize(),
			)
			Expect(err).ToNot(HaveOccurred())
		})

		Describe("FindOrCreateScope", func() {
			Context("given no resource", func() {
				It("finds or creates a global scope", func() {
					createdScope, err := resourceConfig.FindOrCreateScope(nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(createdScope.Resource()).To(BeNil())
					Expect(createdScope.ResourceConfig().ID()).To(Equal(resourceConfig.ID()))

					foundScope, err := resourceConfig.FindOrCreateScope(nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(foundScope.ID()).To(Equal(createdScope.ID()))
				})
			})

			Context("given a resource", func() {
				It("finds or creates a unique scope", func() {
					createdScope, err := resourceConfig.FindOrCreateScope(defaultResource)
					Expect(err).ToNot(HaveOccurred())
					Expect(createdScope.Resource()).ToNot(BeNil())
					Expect(createdScope.Resource().ID()).To(Equal(defaultResource.ID()))
					Expect(createdScope.ResourceConfig().ID()).To(Equal(resourceConfig.ID()))

					foundScope, err := resourceConfig.FindOrCreateScope(defaultResource)
					Expect(err).ToNot(HaveOccurred())
					Expect(foundScope.ID()).To(Equal(createdScope.ID()))
				})
			})
		})
	})
})
