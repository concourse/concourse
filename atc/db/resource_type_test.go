package db_test

import (
	"errors"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceType", func() {
	var pipeline db.Pipeline

	BeforeEach(func() {
		var (
			created bool
			err     error
		)

		pipeline, created, err = defaultTeam.SavePipeline(
			"pipeline-with-types",
			atc.Config{
				ResourceTypes: atc.ResourceTypes{
					{
						Name:   "some-type",
						Type:   "registry-image",
						Source: atc.Source{"some": "repository"},
					},
					{
						Name:       "some-other-type",
						Type:       "registry-image-ng",
						Privileged: true,
						Source:     atc.Source{"some": "other-repository"},
					},
					{
						Name:   "some-type-with-params",
						Type:   "s3",
						Source: atc.Source{"some": "repository"},
						Params: atc.Params{"unpack": "true"},
					},
					{
						Name:       "some-type-with-custom-check",
						Type:       "registry-image",
						Source:     atc.Source{"some": "repository"},
						CheckEvery: "10ms",
					},
				},
			},
			0,
			false,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(created).To(BeTrue())
	})

	Describe("(Pipeline).ResourceTypes", func() {
		var resourceTypes []db.ResourceType

		JustBeforeEach(func() {
			var err error
			resourceTypes, err = pipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns the resource types", func() {
			Expect(resourceTypes).To(HaveLen(4))

			ids := map[int]struct{}{}

			for _, t := range resourceTypes {
				ids[t.ID()] = struct{}{}

				switch t.Name() {
				case "some-type":
					Expect(t.Name()).To(Equal("some-type"))
					Expect(t.Type()).To(Equal("registry-image"))
					Expect(t.Source()).To(Equal(atc.Source{"some": "repository"}))
					Expect(t.Version()).To(BeNil())
				case "some-other-type":
					Expect(t.Name()).To(Equal("some-other-type"))
					Expect(t.Type()).To(Equal("registry-image-ng"))
					Expect(t.Source()).To(Equal(atc.Source{"some": "other-repository"}))
					Expect(t.Version()).To(BeNil())
					Expect(t.Privileged()).To(BeTrue())
				case "some-type-with-params":
					Expect(t.Name()).To(Equal("some-type-with-params"))
					Expect(t.Type()).To(Equal("s3"))
					Expect(t.Params()).To(Equal(atc.Params{"unpack": "true"}))
				case "some-type-with-custom-check":
					Expect(t.Name()).To(Equal("some-type-with-custom-check"))
					Expect(t.Type()).To(Equal("registry-image"))
					Expect(t.Source()).To(Equal(atc.Source{"some": "repository"}))
					Expect(t.Version()).To(BeNil())
					Expect(t.CheckEvery()).To(Equal("10ms"))
				}
			}

			Expect(ids).To(HaveLen(4))
		})

		Context("when a resource type becomes inactive", func() {
			BeforeEach(func() {
				var (
					created bool
					err     error
				)

				pipeline, created, err = defaultTeam.SavePipeline(
					"pipeline-with-types",
					atc.Config{
						ResourceTypes: atc.ResourceTypes{
							{
								Name:   "some-type",
								Type:   "registry-image",
								Source: atc.Source{"some": "repository"},
							},
						},
					},
					pipeline.ConfigVersion(),
					false,
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeFalse())
			})

			It("does not return inactive resource types", func() {
				Expect(resourceTypes).To(HaveLen(1))
				Expect(resourceTypes[0].Name()).To(Equal("some-type"))
			})
		})
	})

	Describe("SetCheckError", func() {
		var resourceType db.ResourceType

		BeforeEach(func() {
			var err error
			resourceType, _, err = pipeline.ResourceType("some-type")
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the resource is first created", func() {
			It("is not errored", func() {
				Expect(resourceType.CheckSetupError()).To(BeNil())
			})
		})

		Context("when a resource check is marked as errored", func() {
			It("is then marked as errored", func() {
				originalCause := errors.New("on fire")

				err := resourceType.SetCheckSetupError(originalCause)
				Expect(err).ToNot(HaveOccurred())

				returnedResourceType, _, err := pipeline.ResourceType("some-type")
				Expect(err).ToNot(HaveOccurred())

				Expect(returnedResourceType.CheckSetupError()).To(Equal(originalCause))
			})
		})

		Context("when a resource is cleared of check errors", func() {
			It("is not marked as errored again", func() {
				originalCause := errors.New("on fire")

				err := resourceType.SetCheckSetupError(originalCause)
				Expect(err).ToNot(HaveOccurred())

				err = resourceType.SetCheckSetupError(nil)
				Expect(err).ToNot(HaveOccurred())

				returnedResourceType, _, err := pipeline.ResourceType("some-type")
				Expect(err).ToNot(HaveOccurred())

				Expect(returnedResourceType.CheckSetupError()).To(BeNil())
			})
		})
	})

	Describe("Resource type version", func() {
		var (
			resourceType      db.ResourceType
			resourceTypeScope db.ResourceConfigScope
		)

		BeforeEach(func() {
			var err error
			resourceType, _, err = pipeline.ResourceType("some-type")
			Expect(err).ToNot(HaveOccurred())
			Expect(resourceType.Version()).To(BeNil())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "registry-image",
			}

			_, err = brt.FindOrCreate(setupTx, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceTypeScope, err = resourceType.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			reloaded, err := resourceType.Reload()
			Expect(reloaded).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates a shared scope for the resource type", func() {
			Expect(resourceTypeScope.Resource()).To(BeNil())
			Expect(resourceTypeScope.ResourceConfig()).ToNot(BeNil())
		})

		Context("when the resource type has proper versions", func() {
			BeforeEach(func() {
				err := resourceTypeScope.SaveVersions([]atc.Version{
					atc.Version{"version": "1"},
					atc.Version{"version": "2"},
				})
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the version", func() {
				Expect(resourceType.Version()).To(Equal(atc.Version{"version": "2"}))
			})
		})
	})
})
