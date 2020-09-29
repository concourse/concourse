package db_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfig", func() {
	Describe("FindResourceConfigScopeByID", func() {
		var pipeline db.Pipeline
		var resourceTypes atc.VersionedResourceTypes

		BeforeEach(func() {
			atc.EnableGlobalResources = true

			config := atc.Config{
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "repository"},
					},
				},
			}

			var created bool
			var err error
			pipeline, created, err = defaultTeam.SavePipeline(
				atc.PipelineRef{Name: "pipeline-one-resource"},
				config,
				0,
				false,
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(BeTrue())

			resourceTypes = atc.VersionedResourceTypes{}
		})

		Context("when a shared resource config scope exists", func() {
			var (
				scope    db.ResourceConfigScope
				resource db.Resource
			)

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "some-type",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				var found bool
				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				scope, err = resource.SetResourceConfig(atc.Source{"some": "repository"}, resourceTypes)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the resource config scope without it scoped to any resource", func() {
				newScope, found, err := scope.ResourceConfig().FindResourceConfigScopeByID(scope.ID(), resource)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(newScope.ID()).To(Equal(scope.ID()))
				Expect(newScope.ResourceConfig().ID()).To(Equal(scope.ResourceConfig().ID()))
				Expect(newScope.Resource()).To(BeNil())
			})
		})

		Context("when a unique resource config scope exists", func() {
			var (
				scope    db.ResourceConfigScope
				resource db.Resource
			)

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "some-type",
				}

				_, err = brt.FindOrCreate(setupTx, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				var found bool
				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				scope, err = resource.SetResourceConfig(atc.Source{"some": "repository"}, resourceTypes)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the resource config scope with it scoped to a resource", func() {
				newScope, found, err := scope.ResourceConfig().FindResourceConfigScopeByID(scope.ID(), resource)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(newScope.ID()).To(Equal(scope.ID()))
				Expect(newScope.ResourceConfig().ID()).To(Equal(scope.ResourceConfig().ID()))
				Expect(newScope.Resource().ID()).To(Equal(resource.ID()))
			})
		})

		Context("when the resource config scope does not exist", func() {
			var (
				resourceConfig db.ResourceConfig
				resource       db.Resource
			)

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "some-type",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig("some-type", atc.Source{"some": "repository"}, resourceTypes)
				Expect(err).ToNot(HaveOccurred())

				var found bool
				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("returns false", func() {
				newScope, found, err := resourceConfig.FindResourceConfigScopeByID(123, resource)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(newScope).To(BeNil())
			})
		})
	})
})
