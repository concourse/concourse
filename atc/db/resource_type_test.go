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
			atc.PipelineRef{Name: "pipeline-with-types"},
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
		var resourceTypes db.ResourceTypes

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
					atc.PipelineRef{Name: "pipeline-with-types"},
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

		Context("when the resource types list has resource with same name", func() {
			BeforeEach(func() {
				var (
					created bool
					err     error
				)

				pipeline, created, err = defaultTeam.SavePipeline(
					atc.PipelineRef{Name: "pipeline-with-types"},
					atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name:   "some-name",
								Type:   "some-name",
								Source: atc.Source{},
							},
						},
						ResourceTypes: atc.ResourceTypes{
							{
								Name:       "some-name",
								Type:       "some-custom-type",
								Source:     atc.Source{"some": "repository"},
								CheckEvery: "10ms",
							},
						},
					},
					pipeline.ConfigVersion(),
					false,
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeFalse())
			})

			It("returns the resource types tree given a resource", func() {
				resource, found, err := pipeline.Resource("some-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				tree := resourceTypes.Filter(resource)
				Expect(len(tree)).To(Equal(1))

				Expect(tree[0].Name()).To(Equal("some-name"))
				Expect(tree[0].Type()).To(Equal("some-custom-type"))
			})
		})

		Context("when building the dependency tree from resource types list", func() {
			BeforeEach(func() {
				var (
					created bool
					err     error
				)

				otherPipeline, created, err := defaultTeam.SavePipeline(
					atc.PipelineRef{Name: "pipeline-with-duplicate-type-name"},
					atc.Config{
						ResourceTypes: atc.ResourceTypes{
							{
								Name:       "some-custom-type",
								Type:       "some-different-foo-type",
								Source:     atc.Source{"some": "repository"},
								CheckEvery: "10ms",
							},
						},
					},
					db.ConfigVersion(0),
					false,
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())
				Expect(otherPipeline).NotTo(BeNil())

				pipeline, created, err = defaultTeam.SavePipeline(
					atc.PipelineRef{Name: "pipeline-with-types"},
					atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name:   "some-resource",
								Type:   "some-custom-type",
								Source: atc.Source{},
							},
						},
						ResourceTypes: atc.ResourceTypes{
							{
								Name:   "registry-image",
								Type:   "registry-image",
								Source: atc.Source{"some": "repository"},
							},
							{
								Name:       "some-other-type",
								Type:       "registry-image",
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
							{
								Name:       "some-custom-type",
								Type:       "some-other-foo-type",
								Source:     atc.Source{"some": "repository"},
								CheckEvery: "10ms",
							},
							{
								Name:       "some-other-foo-type",
								Type:       "some-other-type",
								Source:     atc.Source{"some": "repository"},
								CheckEvery: "10ms",
							},
						},
					},
					pipeline.ConfigVersion(),
					false,
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeFalse())
			})

			It("returns the resource types tree given a resource", func() {
				resource, found, err := pipeline.Resource("some-resource")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				tree := resourceTypes.Filter(resource)
				Expect(len(tree)).To(Equal(4))

				Expect(tree[0].Name()).To(Equal("some-custom-type"))
				Expect(tree[0].Type()).To(Equal("some-other-foo-type"))

				Expect(tree[1].Name()).To(Equal("some-other-foo-type"))
				Expect(tree[1].Type()).To(Equal("some-other-type"))

				Expect(tree[2].Name()).To(Equal("some-other-type"))
				Expect(tree[2].Type()).To(Equal("registry-image"))

				Expect(tree[3].Name()).To(Equal("registry-image"))
				Expect(tree[3].Type()).To(Equal("registry-image"))
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

		It("returns the resource config scope id", func() {
			Expect(resourceType.ResourceConfigScopeID()).To(Equal(resourceTypeScope.ID()))
		})

		Context("when the resource type has proper versions", func() {
			BeforeEach(func() {
				err := resourceTypeScope.SaveVersions(nil, []atc.Version{
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
