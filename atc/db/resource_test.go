package db_test

import (
	"errors"
	"strconv"

	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/algorithm"
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
					Expect(r.ConfigPinnedVersion()).To(Equal(atc.Version{"ref": "abcdef"}))
					Expect(r.CurrentPinnedVersion()).To(Equal(r.ConfigPinnedVersion()))
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
			BeforeEach(func() {
				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the resource", func() {
				Expect(found).To(BeTrue())
				Expect(resource.Name()).To(Equal("some-resource"))
				Expect(resource.Type()).To(Equal("registry-image"))
				Expect(resource.Source()).To(Equal(atc.Source{"some": "repository"}))
			})

			Context("when the resource config id is set on the resource for the first time", func() {
				var resourceScope db.ResourceConfigScope
				var versionsDB *algorithm.VersionsDB

				BeforeEach(func() {
					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "registry-image",
					}

					_, err = brt.FindOrCreate(setupTx, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					versionsDB, err = pipeline.LoadVersionsDB()
					Expect(err).ToNot(HaveOccurred())

					resourceScope, err = resource.SetResourceConfig(logger, atc.Source{"some": "repository"}, creds.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					err = resourceScope.SetCheckError(errors.New("oops"))
					Expect(err).NotTo(HaveOccurred())

					found, err = resource.Reload()
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the resource config check error and bumps the pipeline cache index", func() {
					Expect(found).To(BeTrue())
					Expect(resource.ResourceConfigID()).To(Equal(resourceScope.ResourceConfig().ID()))
					Expect(resource.CheckError()).To(Equal(errors.New("oops")))

					cachedVersionsDB, err := pipeline.LoadVersionsDB()
					Expect(err).ToNot(HaveOccurred())
					Expect(versionsDB != cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be different objects")
				})

				Context("when the resource config id is already set on the resource", func() {
					BeforeEach(func() {
						versionsDB, err = pipeline.LoadVersionsDB()
						Expect(err).ToNot(HaveOccurred())
					})

					It("does not bump the cache index", func() {
						resourceScope, err = resource.SetResourceConfig(logger, atc.Source{"some": "repository"}, creds.VersionedResourceTypes{})
						Expect(err).NotTo(HaveOccurred())

						cachedVersionsDB, err := pipeline.LoadVersionsDB()
						Expect(err).ToNot(HaveOccurred())
						Expect(versionsDB == cachedVersionsDB).To(BeTrue(), "Expected VersionsDB to be the same")
					})
				})
			})

			Context("when the resource config id is not set on the resource", func() {
				It("returns nil for the resource config check error", func() {
					Expect(found).To(BeTrue())
					Expect(resource.ResourceConfigID()).To(Equal(0))
					Expect(resource.CheckError()).To(BeNil())
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

	Describe("SetResourceConfig", func() {
		var pipeline db.Pipeline
		var config atc.Config

		BeforeEach(func() {
			var created bool
			var err error
			config = atc.Config{
				ResourceTypes: atc.ResourceTypes{
					{
						Name:                 "some-resourceType",
						Type:                 "base",
						Source:               atc.Source{"some": "repository"},
						UniqueVersionHistory: true,
					},
				},
				Resources: atc.ResourceConfigs{
					{
						Name:   "some-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "repository"},
					},
					{
						Name:   "some-other-resource",
						Type:   "some-type",
						Source: atc.Source{"some": "repository"},
					},
					{
						Name:   "pipeline-resource",
						Type:   "some-resourceType",
						Source: atc.Source{"some": "repository"},
					},
					{
						Name:   "other-pipeline-resource",
						Type:   "some-resourceType",
						Source: atc.Source{"some": "repository"},
					},
				},
			}

			pipeline, created, err = defaultTeam.SavePipeline(
				"pipeline-with-same-resources",
				config,
				0,
				db.PipelineUnpaused,
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(created).To(BeTrue())
		})

		Context("when the enable global resources flag is set to true", func() {
			BeforeEach(func() {
				atc.EnableGlobalResources = true
			})

			Context("when the resource uses a base resource type with shared version history", func() {
				var (
					resourceScope1 db.ResourceConfigScope
					resourceScope2 db.ResourceConfigScope
					resource1      db.Resource
					resource2      db.Resource
				)

				BeforeEach(func() {
					var found bool
					var err error
					resource1, found, err = pipeline.Resource("some-resource")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "some-type",
					}

					_, err = brt.FindOrCreate(setupTx, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					resourceScope1, err = resource1.SetResourceConfig(logger, atc.Source{"some": "repository"}, creds.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					err = resourceScope1.SetCheckError(errors.New("oops"))
					Expect(err).NotTo(HaveOccurred())

					found, err = resource1.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
				})

				It("has the resource config id and resource config scope id set on the resource", func() {
					Expect(resourceScope1.Resource()).To(BeNil())
					Expect(resource1.ResourceConfigID()).To(Equal(resourceScope1.ResourceConfig().ID()))
					Expect(resource1.ResourceConfigScopeID()).To(Equal(resourceScope1.ID()))
				})

				Context("when another resource uses the same resource config", func() {
					BeforeEach(func() {
						var found bool
						var err error
						resource2, found, err = pipeline.Resource("some-other-resource")
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())

						resourceScope2, err = resource2.SetResourceConfig(logger, atc.Source{"some": "repository"}, creds.VersionedResourceTypes{})
						Expect(err).NotTo(HaveOccurred())

						found, err = resource2.Reload()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
					})

					It("has the same resource config id and resource config scope id as the first resource", func() {
						Expect(resourceScope2.CheckError()).To(Equal(errors.New("oops")))

						Expect(resource2.ResourceConfigID()).To(Equal(resourceScope2.ResourceConfig().ID()))
						Expect(resource2.ResourceConfigScopeID()).To(Equal(resourceScope2.ID()))
						Expect(resource1.ResourceConfigID()).To(Equal(resource2.ResourceConfigID()))
						Expect(resource1.ResourceConfigScopeID()).To(Equal(resource2.ResourceConfigScopeID()))
					})
				})
			})

			Context("when the resource uses a base resource type that has unique version history", func() {
				var (
					resourceScope1 db.ResourceConfigScope
					resourceScope2 db.ResourceConfigScope
					resource1      db.Resource
					resource2      db.Resource
				)

				BeforeEach(func() {
					var found bool
					var err error
					resource1, found, err = pipeline.Resource("some-resource")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "some-type",
					}

					_, err = brt.FindOrCreate(setupTx, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					resourceScope1, err = resource1.SetResourceConfig(logger, atc.Source{"some": "repository"}, creds.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					found, err = resource1.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
				})

				It("has the resource config id and resource config scope id set on the resource", func() {
					Expect(resource1.ResourceConfigID()).To(Equal(resourceScope1.ResourceConfig().ID()))
					Expect(resource1.ResourceConfigScopeID()).To(Equal(resourceScope1.ID()))
				})

				Context("when another resource uses the same resource config", func() {
					BeforeEach(func() {
						var found bool
						var err error
						resource2, found, err = pipeline.Resource("some-other-resource")
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())

						resourceScope2, err = resource2.SetResourceConfig(logger, atc.Source{"some": "repository"}, creds.VersionedResourceTypes{})
						Expect(err).NotTo(HaveOccurred())

						found, err = resource2.Reload()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
					})

					It("has a different resource config scope id than the first resource", func() {
						Expect(resource2.ResourceConfigID()).To(Equal(resourceScope2.ResourceConfig().ID()))
						Expect(resource2.ResourceConfigScopeID()).To(Equal(resourceScope2.ID()))
						Expect(resource1.ResourceConfigID()).To(Equal(resource2.ResourceConfigID()))
						Expect(resource1.ResourceConfigScopeID()).ToNot(Equal(resource2.ResourceConfigScopeID()))
					})
				})
			})

			Context("when the resource uses a resource type that is specified in the pipeline config to have a unique version history", func() {
				var (
					resourceScope1 db.ResourceConfigScope
					resourceScope2 db.ResourceConfigScope
					resource1      db.Resource
					resource2      db.Resource
					resourceTypes  db.ResourceTypes
				)

				BeforeEach(func() {
					var found bool
					var err error
					resource1, found, err = pipeline.Resource("pipeline-resource")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "base",
					}

					_, err = brt.FindOrCreate(setupTx, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					resourceTypes, err = pipeline.ResourceTypes()
					Expect(err).ToNot(HaveOccurred())

					resourceScope1, err = resource1.SetResourceConfig(logger, atc.Source{"some": "repository"}, creds.NewVersionedResourceTypes(template.StaticVariables{}, resourceTypes.Deserialize()))
					Expect(err).NotTo(HaveOccurred())

					found, err = resource1.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
				})

				It("has the resource config id and resource config scope id set on the resource", func() {
					Expect(resourceScope1.Resource()).ToNot(BeNil())
					Expect(resource1.ResourceConfigScopeID()).To(Equal(resourceScope1.ID()))
					Expect(resource1.ResourceConfigID()).To(Equal(resourceScope1.ResourceConfig().ID()))
				})

				Context("when another resource uses the same resource config", func() {
					BeforeEach(func() {
						var found bool
						var err error
						resource2, found, err = pipeline.Resource("other-pipeline-resource")
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())

						resourceScope2, err = resource2.SetResourceConfig(logger, atc.Source{"some": "repository"}, creds.NewVersionedResourceTypes(template.StaticVariables{}, resourceTypes.Deserialize()))
						Expect(err).NotTo(HaveOccurred())

						found, err = resource2.Reload()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
					})

					It("has a different resource config scope id than the first resource", func() {
						Expect(resource2.ResourceConfigID()).To(Equal(resourceScope2.ResourceConfig().ID()))
						Expect(resource1.ResourceConfigScopeID()).ToNot(Equal(resource2.ResourceConfigScopeID()))
					})
				})

				Context("when the resource has a new resource config id and is still unique", func() {
					var newResourceConfigScope db.ResourceConfigScope

					BeforeEach(func() {
						config.Resources[2].Source = atc.Source{"some": "other-repo"}
						newPipeline, _, err := defaultTeam.SavePipeline(
							"pipeline-with-same-resources",
							config,
							pipeline.ConfigVersion(),
							db.PipelineUnpaused,
						)
						Expect(err).ToNot(HaveOccurred())

						var found bool
						resource1, found, err = newPipeline.Resource("pipeline-resource")
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())

						newResourceConfigScope, err = resource1.SetResourceConfig(logger, atc.Source{"some": "other-repo"}, creds.NewVersionedResourceTypes(template.StaticVariables{}, resourceTypes.Deserialize()))
						Expect(err).NotTo(HaveOccurred())

						found, err = resource1.Reload()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
					})

					It("should have a new scope", func() {
						Expect(newResourceConfigScope.ID()).ToNot(Equal(resourceScope1.ID()))
						Expect(resource1.ResourceConfigScopeID()).To(Equal(newResourceConfigScope.ID()))
					})
				})

				Context("when the resource is altered to shared version history", func() {
					var newResourceConfigScope db.ResourceConfigScope

					BeforeEach(func() {
						config.ResourceTypes[0].UniqueVersionHistory = false
						newPipeline, _, err := defaultTeam.SavePipeline(
							"pipeline-with-same-resources",
							config,
							pipeline.ConfigVersion(),
							db.PipelineUnpaused,
						)
						Expect(err).ToNot(HaveOccurred())

						var found bool
						resource1, found, err = newPipeline.Resource("pipeline-resource")
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())

						resourceTypes, err = newPipeline.ResourceTypes()
						Expect(err).ToNot(HaveOccurred())

						newResourceConfigScope, err = resource1.SetResourceConfig(logger, atc.Source{"some": "repository"}, creds.NewVersionedResourceTypes(template.StaticVariables{}, resourceTypes.Deserialize()))
						Expect(err).NotTo(HaveOccurred())

						found, err = resource1.Reload()
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())
					})

					It("should have a new scope", func() {
						Expect(newResourceConfigScope.ID()).ToNot(Equal(resourceScope1.ID()))
						Expect(newResourceConfigScope.Resource()).To(BeNil())
						Expect(resource1.ResourceConfigScopeID()).To(Equal(newResourceConfigScope.ID()))
					})
				})
			})
		})

		Context("when the enable global resources flag is set to false, all resources will have a unique history", func() {
			BeforeEach(func() {
				atc.EnableGlobalResources = false
			})

			Context("when the resource uses a base resource type with shared version history", func() {
				var (
					resource1 db.Resource
					resource2 db.Resource
				)

				BeforeEach(func() {
					var found bool
					var err error
					resource1, found, err = pipeline.Resource("some-resource")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					resource2, found, err = pipeline.Resource("some-other-resource")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "some-type",
					}

					_, err = brt.FindOrCreate(setupTx, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					_, err = resource1.SetResourceConfig(logger, atc.Source{"some": "repository"}, creds.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					_, err = resource2.SetResourceConfig(logger, atc.Source{"some": "repository"}, creds.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					found, err = resource1.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					found, err = resource2.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
				})

				It("has unique version histories", func() {
					Expect(resource1.ResourceConfigScopeID()).ToNot(Equal(resource2.ResourceConfigScopeID()))
				})
			})
		})
	})

	Describe("SetCheckSetupError", func() {
		var resource db.Resource

		BeforeEach(func() {
			var err error
			resource, _, err = pipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the resource is first created", func() {
			It("is not errored", func() {
				Expect(resource.CheckSetupError()).To(BeNil())
			})
		})

		Context("when a resource check is marked as errored", func() {
			It("is then marked as errored", func() {
				originalCause := errors.New("on fire")

				err := resource.SetCheckSetupError(originalCause)
				Expect(err).ToNot(HaveOccurred())

				returnedResource, _, err := pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())

				Expect(returnedResource.CheckSetupError()).To(Equal(originalCause))
			})
		})

		Context("when a resource is cleared of check errors", func() {
			It("is not marked as errored again", func() {
				originalCause := errors.New("on fire")

				err := resource.SetCheckSetupError(originalCause)
				Expect(err).ToNot(HaveOccurred())

				err = resource.SetCheckSetupError(nil)
				Expect(err).ToNot(HaveOccurred())

				returnedResource, _, err := pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())

				Expect(returnedResource.CheckSetupError()).To(BeNil())
			})
		})
	})

	Describe("ResourceVersion", func() {
		var (
			resource             db.Resource
			version              atc.Version
			rcvID                int
			resourceVersionFound bool
			foundErr             error
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
			rcvID, resourceVersionFound, foundErr = resource.ResourceVersionID(version)
		})

		Context("when the version exists", func() {
			var (
				resourceVersion db.ResourceVersion
				resourceScope   db.ResourceConfigScope
			)

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "registry-image",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				resourceScope, err = resource.SetResourceConfig(logger, atc.Source{"some": "repository"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				saveVersions(resourceScope, []atc.SpaceVersion{
					{
						Space:   atc.Space("space"),
						Version: version,
					},
				})

				var found bool
				resourceVersion, found, err = resourceScope.FindVersion(atc.Space("space"), version)
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns resource config version and true", func() {
				Expect(resourceVersionFound).To(BeTrue())
				Expect(rcvID).To(Equal(resourceVersion.ID()))
				Expect(foundErr).ToNot(HaveOccurred())
			})

			Context("when the check order is 0", func() {
				BeforeEach(func() {
					version = atc.Version{"version": "2"}
					created, err := resourceScope.SaveUncheckedVersion(atc.Space("space"), version, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(created).To(BeTrue())
				})

				It("does not find the resource config version", func() {
					Expect(rcvID).To(Equal(0))
					Expect(resourceVersionFound).To(BeFalse())
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

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())
				_, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "registry-image", atc.Source{"some": "repository"}, creds.VersionedResourceTypes{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns false when resourceConfig is not found", func() {
				Expect(foundErr).ToNot(HaveOccurred())
				Expect(resourceVersionFound).To(BeFalse())
			})
		})
	})

	Context("Versions", func() {
		var (
			originalVersionSlice []atc.SpaceVersion
			resource             db.Resource
			resourceScope        db.ResourceConfigScope
		)

		Context("when resource has versions created in order of check order", func() {
			var resourceVersions []atc.ResourceVersion

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "git",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				var found bool
				resource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceScope, err = resource.SetResourceConfig(logger, atc.Source{"some": "other-repository"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				originalVersionSlice = []atc.SpaceVersion{
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v0"},
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v1"},
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v2"},
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v3"},
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v4"},
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v5"},
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v6"},
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v7"},
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v8"},
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v9"},
					},
				}

				saveVersions(resourceScope, originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				resourceVersions = make([]atc.ResourceVersion, 0)

				for i := 0; i < 10; i++ {
					rcv, found, err := resourceScope.FindVersion(atc.Space("space"), atc.Version{"ref": "v" + strconv.Itoa(i)})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					var metadata []atc.MetadataField

					for _, v := range rcv.Metadata() {
						metadata = append(metadata, atc.MetadataField(v))
					}

					resourceVersion := atc.ResourceVersion{
						ID:       rcv.ID(),
						Version:  atc.Version(rcv.Version()),
						Metadata: metadata,
						Enabled:  true,
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
					_, err := resourceScope.SaveUncheckedVersion(atc.Space("space"), atc.Version(resourceVersions[9].Version), metadata)
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

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				var found bool
				resource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceScope, err = resource.SetResourceConfig(logger, atc.Source{"some": "other-repository"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				originalVersionSlice := []atc.SpaceVersion{
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v1"}, // id: 1, check_order: 1
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v3"}, // id: 2, check_order: 2
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v4"}, // id: 3, check_order: 3
					},
				}

				saveVersions(resourceScope, originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				secondVersionSlice := []atc.SpaceVersion{
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v2"}, // id: 4, check_order: 4
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v3"}, // id: 2, check_order: 5
					},
					{
						Space:   atc.Space("space"),
						Version: atc.Version{"ref": "v4"}, // id: 3, check_order: 6
					},
				}

				saveVersions(resourceScope, secondVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				for i := 1; i < 5; i++ {
					rcv, found, err := resourceScope.FindVersion(atc.Space("space"), atc.Version{"ref": "v" + strconv.Itoa(i)})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					var metadata []atc.MetadataField

					for _, v := range rcv.Metadata() {
						metadata = append(metadata, atc.MetadataField(v))
					}

					resourceVersion := atc.ResourceVersion{
						ID:       rcv.ID(),
						Version:  atc.Version(rcv.Version()),
						Metadata: metadata,
						Enabled:  true,
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

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				var found bool
				resource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				resourceScope, err := resource.SetResourceConfig(logger, atc.Source{"some": "other-repository"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = resourceScope.SaveSpace(atc.Space("space"))
				Expect(err).ToNot(HaveOccurred())

				created, err := resourceScope.SaveUncheckedVersion(atc.Space("space"), atc.Version{"version": "not-returned"}, nil)
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

	Describe("PinVersion/UnpinVersion", func() {
		var resource db.Resource
		var resID int

		BeforeEach(func() {
			var found bool
			var err error
			resource, found, err = pipeline.Resource("some-other-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "git",
			}

			_, err = brt.FindOrCreate(setupTx, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceScope, err := resource.SetResourceConfig(logger, atc.Source{"some": "other-repository"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			err = resource.SetResourceConfig(resourceConfig.ID())
			Expect(err).ToNot(HaveOccurred())

			saveVersions(resourceScope, []atc.SpaceVersion{
				atc.SpaceVersion{
					Space:   atc.Space("space"),
					Version: atc.Version{"version": "v1"},
				},
				atc.SpaceVersion{
					Space:   atc.Space("space"),
					Version: atc.Version{"version": "v2"},
				},
				atc.SpaceVersion{
					Space:   atc.Space("space"),
					Version: atc.Version{"version": "v3"},
				},
			})

			resConf, found, err := resourceScope.FindVersion(atc.Space("space"), atc.Version{"version": "v1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			resID = resConf.ID()
		})

		// XXX: FIX PINNING
		XContext("when we pin a resource to a version", func() {
			BeforeEach(func() {
				err := resource.PinVersion(resID)
				Expect(err).ToNot(HaveOccurred())

				found, err := resource.Reload()
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})

			It("sets the api pinned version", func() {
				Expect(resource.APIPinnedVersion()).To(Equal(atc.Version{"version": "v1"}))
				Expect(resource.CurrentPinnedVersion()).To(Equal(resource.APIPinnedVersion()))
			})

			Context("when we set the pin comment on a resource", func() {
				BeforeEach(func() {
					err := resource.SetPinComment("foo")
					Expect(err).ToNot(HaveOccurred())
					resource, _, err = pipeline.Resource("some-other-resource")
					Expect(err).ToNot(HaveOccurred())
				})

				It("should set the pin comment", func() {
					Expect(resource.PinComment()).To(Equal("foo"))
				})

				Context("when we unpin a resource to a version", func() {
					BeforeEach(func() {
						err := resource.UnpinVersion()
						Expect(err).ToNot(HaveOccurred())

						found, err := resource.Reload()
						Expect(found).To(BeTrue())
						Expect(err).ToNot(HaveOccurred())
					})

					It("sets the api pinned version to nil", func() {
						Expect(resource.APIPinnedVersion()).To(BeNil())
						Expect(resource.CurrentPinnedVersion()).To(BeNil())
					})

					It("unsets the pin comment", func() {
						Expect(resource.PinComment()).To(BeEmpty())
					})
				})
			})
		})

		Context("when we pin a resource that is already pinned to a version (through the config)", func() {
			BeforeEach(func() {
				var found bool
				var err error
				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "registry-image",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				resourceScope, err := resource.SetResourceConfig(logger, atc.Source{"some": "repository"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				saveVersions(resourceScope, []atc.SpaceVersion{
					atc.SpaceVersion{
						Space:   atc.Space("space"),
						Version: atc.Version{"version": "v1"},
					},
					atc.SpaceVersion{
						Space:   atc.Space("space"),
						Version: atc.Version{"version": "v2"},
					},
					atc.SpaceVersion{
						Space:   atc.Space("space"),
						Version: atc.Version{"version": "v3"},
					},
				})

				resConf, found, err := resourceScope.FindVersion(atc.Space("space"), atc.Version{"version": "v1"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = resource.PinVersion(resConf.ID())
				Expect(err).ToNot(HaveOccurred())

				found, err = resource.Reload()
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return the config pinned version", func() {
				Expect(resource.CurrentPinnedVersion()).To(Equal(atc.Version{"ref": "abcdef"}))
			})
		})
	})
})
