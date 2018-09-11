package db_test

import (
	"errors"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource", func() {
	var pipeline db.Pipeline

	BeforeEach(func() {
		var (
			created bool
			err     error
		)

		pipeline, created, err = defaultTeam.SavePipeline(
			"pipeline-with-resources",
			atc.Config{
				Resources: atc.ResourceConfigs{
					{
						Name:    "some-resource",
						Type:    "registry-image",
						Source:  atc.Source{"some": "repository"},
						Version: atc.Version{"ref": "abcdef"},
					},
					{
						Name:   "some-other-resource",
						Type:   "git",
						Source: atc.Source{"some": "other-repository"},
					},
					{
						Name:   "some-secret-resource",
						Type:   "git",
						Source: atc.Source{"some": "((secret-repository))"},
					},
					{
						Name:         "some-resource-custom-check",
						Type:         "git",
						Source:       atc.Source{"some": "some-repository"},
						CheckEvery:   "10ms",
						CheckTimeout: "1m",
					},
				},
			},
			0,
			db.PipelineUnpaused,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(created).To(BeTrue())
	})

	Describe("(Pipeline).Resources", func() {
		var resources []db.Resource

		JustBeforeEach(func() {
			var err error
			resources, err = pipeline.Resources()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns the resources", func() {
			Expect(resources).To(HaveLen(4))

			ids := map[int]struct{}{}

			for _, r := range resources {
				ids[r.ID()] = struct{}{}

				switch r.Name() {
				case "some-resource":
					Expect(r.Type()).To(Equal("registry-image"))
					Expect(r.Source()).To(Equal(atc.Source{"some": "repository"}))
					Expect(r.PinnedVersion()).To(Equal(atc.Version{"ref": "abcdef"}))
				case "some-other-resource":
					Expect(r.Type()).To(Equal("git"))
					Expect(r.Source()).To(Equal(atc.Source{"some": "other-repository"}))
				case "some-secret-resource":
					Expect(r.Type()).To(Equal("git"))
					Expect(r.Source()).To(Equal(atc.Source{"some": "((secret-repository))"}))
				case "some-resource-custom-check":
					Expect(r.Type()).To(Equal("git"))
					Expect(r.Source()).To(Equal(atc.Source{"some": "some-repository"}))
					Expect(r.CheckEvery()).To(Equal("10ms"))
					Expect(r.CheckTimeout()).To(Equal("1m"))
				}
			}
		})
	})

	Describe("(Pipeline).Resource", func() {
		var (
			err      error
			found    bool
			resource db.Resource
		)

		Context("when the resource exists", func() {
			JustBeforeEach(func() {
				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the resource", func() {
				Expect(found).To(BeTrue())
				Expect(resource.Name()).To(Equal("some-resource"))
				Expect(resource.Type()).To(Equal("registry-image"))
				Expect(resource.Source()).To(Equal(atc.Source{"some": "repository"}))
			})

			Context("when the resource config id is set on the resource", func() {
				var resourceConfig db.ResourceConfig

				BeforeEach(func() {
					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "registry-image",
					}
					_, err = brt.FindOrCreate(setupTx)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "registry-image", atc.Source{"some": "repository"}, creds.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					err = resourceConfig.SetCheckError(errors.New("oops"))
					Expect(err).NotTo(HaveOccurred())
				})

				JustBeforeEach(func() {
					err = resource.SetResourceConfig(resourceConfig.ID())
					Expect(err).NotTo(HaveOccurred())

					found, err = resource.Reload()
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the resource config check error", func() {
					Expect(found).To(BeTrue())
					Expect(resource.ResourceConfigCheckError()).To(Equal(errors.New("oops")))
				})
			})

			Context("when the resource config id is not set on the resource", func() {
				It("returns nil for the resource config check error", func() {
					Expect(found).To(BeTrue())
					Expect(resource.ResourceConfigCheckError()).To(BeNil())
				})
			})
		})

		Context("when the resource does not exist", func() {
			JustBeforeEach(func() {
				resource, found, err = pipeline.Resource("bonkers")
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns nil", func() {
				Expect(found).To(BeFalse())
				Expect(resource).To(BeNil())
			})
		})
	})

	Describe("Pause", func() {
		var (
			resource db.Resource
			err      error
			found    bool
		)

		JustBeforeEach(func() {
			resource, found, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.Paused()).To(BeFalse())
		})

		It("pauses the resource", func() {
			err = resource.Pause()
			Expect(err).ToNot(HaveOccurred())

			found, err = resource.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.Paused()).To(BeTrue())
		})
	})

	Describe("Unpause", func() {
		var (
			resource db.Resource
			err      error
			found    bool
		)

		JustBeforeEach(func() {
			resource, found, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = resource.Pause()
			Expect(err).ToNot(HaveOccurred())

			found, err = resource.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.Paused()).To(BeTrue())
		})

		It("pauses the resource", func() {
			err = resource.Unpause()
			Expect(err).ToNot(HaveOccurred())

			found, err = resource.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(resource.Paused()).To(BeFalse())
		})
	})

	Describe("SetCheckError", func() {
		var resource db.Resource

		BeforeEach(func() {
			var err error
			resource, _, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the resource is first created", func() {
			It("is not errored", func() {
				Expect(resource.CheckError()).To(BeNil())
			})
		})

		Context("when a resource check is marked as errored", func() {
			It("is then marked as errored", func() {
				originalCause := errors.New("on fire")

				err := resource.SetCheckError(originalCause)
				Expect(err).ToNot(HaveOccurred())

				returnedResource, _, err := pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())

				Expect(returnedResource.CheckError()).To(Equal(originalCause))
			})
		})

		Context("when a resource is cleared of check errors", func() {
			It("is not marked as errored again", func() {
				originalCause := errors.New("on fire")

				err := resource.SetCheckError(originalCause)
				Expect(err).ToNot(HaveOccurred())

				err = resource.SetCheckError(nil)
				Expect(err).ToNot(HaveOccurred())

				returnedResource, _, err := pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())

				Expect(returnedResource.CheckError()).To(BeNil())
			})
		})
	})

	Describe("ResourceConfigVersion", func() {
		var (
			resource                   db.Resource
			version                    atc.Version
			rcvID                      int
			resourceConfigVersionFound bool
			foundErr                   error
		)

		BeforeEach(func() {
			var err error
			var found bool
			version = atc.Version{"version": "12345"}
			resource, found, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		JustBeforeEach(func() {
			rcvID, resourceConfigVersionFound, foundErr = resource.ResourceConfigVersionID(version)
		})

		Context("when the version exists", func() {
			var (
				resourceConfigVersion db.ResourceConfigVersion
				resourceConfig        db.ResourceConfig
			)

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "registry-image",
				}
				_, err = brt.FindOrCreate(setupTx)
				Expect(err).ToNot(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "registry-image", atc.Source{"some": "repository"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = resourceConfig.SaveVersions([]atc.Version{version})
				Expect(err).ToNot(HaveOccurred())

				err = resource.SetResourceConfig(resourceConfig.ID())
				Expect(err).ToNot(HaveOccurred())

				var found bool
				resourceConfigVersion, found, err = resourceConfig.FindVersion(version)
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns resource config version and true", func() {
				Expect(resourceConfigVersionFound).To(BeTrue())
				Expect(rcvID).To(Equal(resourceConfigVersion.ID()))
				Expect(foundErr).ToNot(HaveOccurred())
			})

			Context("when the check order is 0", func() {
				BeforeEach(func() {
					version = atc.Version{"version": "2"}
					created, err := resourceConfig.SaveVersion(version, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(created).To(BeTrue())
				})

				It("does not find the resource config version", func() {
					Expect(rcvID).To(Equal(0))
					Expect(resourceConfigVersionFound).To(BeFalse())
					Expect(foundErr).ToNot(HaveOccurred())
				})
			})
		})

		Context("when the version is not found", func() {
			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "registry-image",
				}
				_, err = brt.FindOrCreate(setupTx)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())
				_, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "registry-image", atc.Source{"some": "repository"}, creds.VersionedResourceTypes{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns false when resourceConfig is not found", func() {
				Expect(foundErr).ToNot(HaveOccurred())
				Expect(resourceConfigVersionFound).To(BeFalse())
			})
		})
	})
})
