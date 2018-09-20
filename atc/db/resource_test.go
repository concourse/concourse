package db_test

import (
	"errors"
	"strconv"

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
					Expect(resource.ResourceConfigID()).To(Equal(resourceConfig.ID()))
					Expect(resource.ResourceConfigCheckError()).To(Equal(errors.New("oops")))
				})
			})

			Context("when the resource config id is not set on the resource", func() {
				It("returns nil for the resource config check error", func() {
					Expect(found).To(BeTrue())
					Expect(resource.ResourceConfigID()).To(Equal(0))
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

	Context("Versions", func() {
		var (
			originalVersionSlice []atc.Version
			resource             db.Resource
			resourceConfig       db.ResourceConfig
		)

		Context("when resource has versions created in order of check order", func() {
			var resourceVersions []atc.ResourceVersion

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "git",
				}
				_, err = brt.FindOrCreate(setupTx)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				var found bool
				resource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "git", atc.Source{"some": "other-repository"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = resource.SetResourceConfig(resourceConfig.ID())
				Expect(err).ToNot(HaveOccurred())

				originalVersionSlice = []atc.Version{
					{"ref": "v0"},
					{"ref": "v1"},
					{"ref": "v2"},
					{"ref": "v3"},
					{"ref": "v4"},
					{"ref": "v5"},
					{"ref": "v6"},
					{"ref": "v7"},
					{"ref": "v8"},
					{"ref": "v9"},
				}

				err = resourceConfig.SaveVersions(originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				resourceVersions = make([]atc.ResourceVersion, 0)

				for i := 0; i < 10; i++ {
					rcv, found, err := resourceConfig.FindVersion(atc.Version{"ref": "v" + strconv.Itoa(i)})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					var metadata []atc.MetadataField

					for _, v := range rcv.Metadata() {
						metadata = append(metadata, atc.MetadataField(v))
					}

					disabled, err := resource.IsVersionDisabled(atc.Version(rcv.Version()))
					Expect(err).ToNot(HaveOccurred())

					resourceVersion := atc.ResourceVersion{
						ID:       rcv.ID(),
						Version:  atc.Version(rcv.Version()),
						Metadata: metadata,
						Enabled:  !disabled,
					}

					resourceVersions = append(resourceVersions, resourceVersion)
				}
			})

			Context("with no since/until", func() {
				It("returns the first page, with the given limit, and a next page", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[8].Version))
					Expect(pagination.Previous).To(BeNil())
					Expect(pagination.Next).To(Equal(&db.Page{Since: resourceVersions[8].ID, Limit: 2}))
				})
			})

			Context("with a since that places it in the middle of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{Since: resourceVersions[6].ID, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[5].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[4].Version))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: resourceVersions[5].ID, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: resourceVersions[4].ID, Limit: 2}))
				})
			})

			Context("with a since that places it at the end of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{Since: resourceVersions[2].ID, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[1].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[0].Version))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: resourceVersions[1].ID, Limit: 2}))
					Expect(pagination.Next).To(BeNil())
				})
			})

			Context("with an until that places it in the middle of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{Until: resourceVersions[6].ID, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[8].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[7].Version))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: resourceVersions[8].ID, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: resourceVersions[7].ID, Limit: 2}))
				})
			})

			Context("with a until that places it at the beginning of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{Until: resourceVersions[7].ID, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[8].Version))
					Expect(pagination.Previous).To(BeNil())
					Expect(pagination.Next).To(Equal(&db.Page{Since: resourceVersions[8].ID, Limit: 2}))
				})
			})

			Context("when the version has metadata", func() {
				BeforeEach(func() {
					metadata := []db.ResourceConfigMetadataField{{Name: "name1", Value: "value1"}}

					// save metadata
					_, err := resourceConfig.SaveVersion(atc.Version(resourceVersions[9].Version), metadata)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns the metadata in the version history", func() {
					historyPage, _, found, err := resource.Versions(db.Page{Limit: 1})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(1))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
				})
			})

			Context("when a version is disabled", func() {
				BeforeEach(func() {
					err := resource.DisableVersion(resourceVersions[9].ID)
					Expect(err).ToNot(HaveOccurred())

					resourceVersions[9].Enabled = false
				})

				It("returns a disabled version", func() {
					historyPage, _, found, err := resource.Versions(db.Page{Limit: 1})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(ConsistOf([]atc.ResourceVersion{resourceVersions[9]}))
				})
			})
		})

		Context("when check orders are different than versions ids", func() {
			var resourceVersions []atc.ResourceVersion

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "git",
				}
				_, err = brt.FindOrCreate(setupTx)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				var found bool
				resource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceConfigFactory := db.NewResourceConfigFactory(dbConn, lockFactory)
				resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "git", atc.Source{"some": "other-repository"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = resource.SetResourceConfig(resourceConfig.ID())
				Expect(err).ToNot(HaveOccurred())

				originalVersionSlice := []atc.Version{
					{"ref": "v1"}, // id: 1, check_order: 1
					{"ref": "v3"}, // id: 2, check_order: 2
					{"ref": "v4"}, // id: 3, check_order: 3
				}

				err = resourceConfig.SaveVersions(originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				secondVersionSlice := []atc.Version{
					{"ref": "v2"}, // id: 4, check_order: 4
					{"ref": "v3"}, // id: 2, check_order: 5
					{"ref": "v4"}, // id: 3, check_order: 6
				}

				err = resourceConfig.SaveVersions(secondVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				for i := 1; i < 5; i++ {
					rcv, found, err := resourceConfig.FindVersion(atc.Version{"ref": "v" + strconv.Itoa(i)})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					var metadata []atc.MetadataField

					for _, v := range rcv.Metadata() {
						metadata = append(metadata, atc.MetadataField(v))
					}

					disabled, err := resource.IsVersionDisabled(atc.Version(rcv.Version()))
					Expect(err).ToNot(HaveOccurred())

					resourceVersion := atc.ResourceVersion{
						ID:       rcv.ID(),
						Version:  atc.Version(rcv.Version()),
						Metadata: metadata,
						Enabled:  !disabled,
					}

					resourceVersions = append(resourceVersions, resourceVersion)
				}

				// ids ordered by check order now: [3, 2, 4, 1]
			})

			Context("with no since/until", func() {
				It("returns versions ordered by check order", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{Limit: 4})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(4))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[3].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[2].Version))
					Expect(historyPage[2].Version).To(Equal(resourceVersions[1].Version))
					Expect(historyPage[3].Version).To(Equal(resourceVersions[0].Version))
					Expect(pagination.Previous).To(BeNil())
					Expect(pagination.Next).To(BeNil())
				})
			})

			Context("with a since", func() {
				It("returns the builds, with previous/next pages excluding since", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{Since: 3, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[2].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[1].Version))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: 4, Limit: 2}))
				})
			})

			Context("with from", func() {
				It("returns the builds, with previous/next pages including from", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{From: 2, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[2].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[1].Version))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: 4, Limit: 2}))
				})
			})

			Context("with a until", func() {
				It("returns the builds, with previous/next pages excluding until", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{Until: 1, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[2].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[1].Version))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: 4, Limit: 2}))
				})
			})

			Context("with to", func() {
				It("returns the builds, with previous/next pages including to", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{To: 4, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[2].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[1].Version))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: 4, Limit: 2}))
				})
			})
		})

		Context("when resource has a version with check order of 0", func() {
			var resource db.Resource

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "git",
				}
				_, err = brt.FindOrCreate(setupTx)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "git", atc.Source{"some": "other-repository"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				var found bool
				resource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = resource.SetResourceConfig(resourceConfig.ID())
				Expect(err).ToNot(HaveOccurred())

				created, err := resourceConfig.SaveVersion(atc.Version{"version": "not-returned"}, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())
			})

			It("does not return the version", func() {
				historyPage, pagination, found, err := resource.Versions(db.Page{Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(historyPage).To(BeNil())
				Expect(pagination).To(Equal(db.Pagination{Previous: nil, Next: nil}))
			})
		})
	})

	Describe("IsVersionDisabled", func() {
		version := atc.Version{"version": "some-version"}
		var resourceConfig db.ResourceConfig
		var resource db.Resource

		BeforeEach(func() {
			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "git",
			}
			_, err = brt.FindOrCreate(setupTx)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "git", atc.Source{"some": "other-repository"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			err = resourceConfig.SaveVersions([]atc.Version{version})
			Expect(err).ToNot(HaveOccurred())

			var found bool
			resource, found, err = pipeline.Resource("some-other-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = resource.SetResourceConfig(resourceConfig.ID())
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the resource is enabled", func() {
			It("should return false", func() {
				isResourceDisabled, err := resource.IsVersionDisabled(version)
				Expect(err).ToNot(HaveOccurred())
				Expect(isResourceDisabled).To(BeFalse())
			})
		})

		Context("when the resource is disabled", func() {
			BeforeEach(func() {
				rcv, found, err := resourceConfig.FindVersion(version)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = resource.DisableVersion(rcv.ID())
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return true", func() {
				isResourceDisabled, err := resource.IsVersionDisabled(version)
				Expect(err).ToNot(HaveOccurred())
				Expect(isResourceDisabled).To(BeTrue())
			})
		})
	})

	Describe("EnableVersion/DisableVersion", func() {
		version := atc.Version{"version": "some-version"}
		var resourceConfig db.ResourceConfig
		var resource db.Resource

		BeforeEach(func() {
			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "git",
			}
			_, err = brt.FindOrCreate(setupTx)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "git", atc.Source{"some": "other-repository"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			err = resourceConfig.SaveVersions([]atc.Version{version})
			Expect(err).ToNot(HaveOccurred())

			var found bool
			resource, found, err = pipeline.Resource("some-other-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			err = resource.SetResourceConfig(resourceConfig.ID())
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when disabling the resource version", func() {
			BeforeEach(func() {
				err := resource.DisableVersion(resourceConfig.ID())
				Expect(err).ToNot(HaveOccurred())
			})

			It("should disable the version", func() {
				Expect(resource.IsVersionDisabled(version)).To(BeTrue())
			})

			Context("when enabling the resource version", func() {
				It("should enable the version", func() {
					err := resource.EnableVersion(resourceConfig.ID())
					Expect(err).ToNot(HaveOccurred())
					Expect(resource.IsVersionDisabled(version)).To(BeFalse())
				})
			})
		})
	})
})
