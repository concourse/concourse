package db_test

import (
	"errors"
	"strconv"
	"time"

	"github.com/concourse/concourse/atc"
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
						Name:         "some-resource",
						Type:         "registry-image",
						WebhookToken: "some-token",
						Source:       atc.Source{"some": "repository"},
						Version:      atc.Version{"ref": "abcdef"},
					},
					{
						Name:   "some-other-resource",
						Public: true,
						Type:   "git",
						Source: atc.Source{"some": "other-repository"},
					},
					{
						Name:   "some-secret-resource",
						Public: false,
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
				Jobs: atc.JobConfigs{
					{
						Name: "job-using-resource",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name: "some-other-resource",
								},
							},
						},
					},
					{
						Name: "not-using-resource",
					},
				},
			},
			0,
			false,
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
					Expect(r.HasWebhook()).To(BeTrue())
				case "some-other-resource":
					Expect(r.Type()).To(Equal("git"))
					Expect(r.Source()).To(Equal(atc.Source{"some": "other-repository"}))
					Expect(r.HasWebhook()).To(BeFalse())
				case "some-secret-resource":
					Expect(r.Type()).To(Equal("git"))
					Expect(r.Source()).To(Equal(atc.Source{"some": "((secret-repository))"}))
					Expect(r.HasWebhook()).To(BeFalse())
				case "some-resource-custom-check":
					Expect(r.Type()).To(Equal("git"))
					Expect(r.Source()).To(Equal(atc.Source{"some": "some-repository"}))
					Expect(r.CheckEvery()).To(Equal("10ms"))
					Expect(r.CheckTimeout()).To(Equal("1m"))
					Expect(r.HasWebhook()).To(BeFalse())
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

				BeforeEach(func() {
					setupTx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					brt := db.BaseResourceType{
						Name: "registry-image",
					}

					_, err = brt.FindOrCreate(setupTx, false)
					Expect(err).NotTo(HaveOccurred())
					Expect(setupTx.Commit()).To(Succeed())

					resourceScope, err = resource.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					err = resourceScope.SetCheckError(errors.New("oops"))
					Expect(err).NotTo(HaveOccurred())

					found, err = resource.Reload()
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the resource config check error", func() {
					Expect(found).To(BeTrue())
					Expect(resource.ResourceConfigID()).To(Equal(resourceScope.ResourceConfig().ID()))
					Expect(resource.CheckError()).To(Equal(errors.New("oops")))
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

	Describe("Enable/Disable Version", func() {
		var resource db.Resource
		var rcv db.ResourceConfigVersion
		var err error
		var found bool

		BeforeEach(func() {
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

			resourceScope, err := resource.SetResourceConfig(atc.Source{"some": "other-repository"}, atc.VersionedResourceTypes{})
			Expect(err).NotTo(HaveOccurred())

			err = resourceScope.SaveVersions(nil, []atc.Version{
				{"disabled": "version"},
			})
			Expect(err).ToNot(HaveOccurred())

			rcv, found, err = resourceScope.FindVersion(atc.Version{"disabled": "version"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when disabling a version that exists", func() {
			var disableErr error
			var requestedSchedule1 time.Time
			var requestedSchedule2 time.Time
			var jobUsingResource db.Job
			var jobNotUsingResource db.Job

			BeforeEach(func() {
				jobUsingResource, found, err = pipeline.Job("job-using-resource")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				requestedSchedule1 = jobUsingResource.ScheduleRequestedTime()

				jobNotUsingResource, found, err = pipeline.Job("not-using-resource")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				requestedSchedule2 = jobNotUsingResource.ScheduleRequestedTime()

				disableErr = resource.DisableVersion(rcv.ID())
			})

			It("successfully disables the version", func() {
				Expect(disableErr).ToNot(HaveOccurred())

				versions, _, found, err := resource.Versions(db.Page{Limit: 3}, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(versions).To(HaveLen(1))
				Expect(versions[0].Version).To(Equal(atc.Version{"disabled": "version"}))
				Expect(versions[0].Enabled).To(BeFalse())
			})

			It("requests schedule on the jobs using that resource", func() {
				found, err := jobUsingResource.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(jobUsingResource.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule1))
			})

			It("does not request schedule on jobs that do not use the resource", func() {
				found, err := jobNotUsingResource.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(jobNotUsingResource.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule2))
			})

			Context("when enabling that version", func() {
				var enableErr error
				BeforeEach(func() {
					enableErr = resource.EnableVersion(rcv.ID())
				})

				It("successfully enables the version", func() {
					Expect(enableErr).ToNot(HaveOccurred())

					versions, _, found, err := resource.Versions(db.Page{Limit: 3}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(versions).To(HaveLen(1))
					Expect(versions[0].Version).To(Equal(atc.Version{"disabled": "version"}))
					Expect(versions[0].Enabled).To(BeTrue())
				})

				It("request schedule on the jobs using that resource", func() {
					found, err := jobUsingResource.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(jobUsingResource.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule1))
				})

				It("does not request schedule on jobs that do not use the resource", func() {
					found, err := jobNotUsingResource.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(jobNotUsingResource.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule2))
				})
			})
		})

		Context("when disabling version that does not exist", func() {
			var disableErr error
			BeforeEach(func() {
				resource, found, err := pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				disableErr = resource.DisableVersion(123456)
			})

			It("returns an error", func() {
				Expect(disableErr).To(HaveOccurred())
			})
		})

		Context("when enabling a version that is already enabled", func() {
			var enableError error
			BeforeEach(func() {
				enableError = resource.EnableVersion(rcv.ID())
			})

			It("returns a non one row affected error", func() {
				Expect(enableError).To(Equal(db.NonOneRowAffectedError{0}))
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
				Jobs: atc.JobConfigs{
					{
						Name: "job-using-resource",
						PlanSequence: []atc.Step{
							{
								Config: &atc.GetStep{
									Name: "some-resource",
								},
							},
						},
					},
					{
						Name: "not-using-resource",
					},
				},
			}

			pipeline, created, err = defaultTeam.SavePipeline(
				"pipeline-with-same-resources",
				config,
				0,
				false,
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

					resourceScope1, err = resource1.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
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

						resourceScope2, err = resource2.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
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

					resourceScope1, err = resource1.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
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

						resourceScope2, err = resource2.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
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

					resourceScope1, err = resource1.SetResourceConfig(atc.Source{"some": "repository"}, resourceTypes.Deserialize())
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

						resourceScope2, err = resource2.SetResourceConfig(atc.Source{"some": "repository"}, resourceTypes.Deserialize())
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
							false,
						)
						Expect(err).ToNot(HaveOccurred())

						var found bool
						resource1, found, err = newPipeline.Resource("pipeline-resource")
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())

						newResourceConfigScope, err = resource1.SetResourceConfig(atc.Source{"some": "other-repo"}, resourceTypes.Deserialize())
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
							false,
						)
						Expect(err).ToNot(HaveOccurred())

						var found bool
						resource1, found, err = newPipeline.Resource("pipeline-resource")
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())

						resourceTypes, err = newPipeline.ResourceTypes()
						Expect(err).ToNot(HaveOccurred())

						newResourceConfigScope, err = resource1.SetResourceConfig(atc.Source{"some": "repository"}, resourceTypes.Deserialize())
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

					_, err = resource1.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
					Expect(err).NotTo(HaveOccurred())

					_, err = resource2.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
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

		Context("when requesting schedule for setting resource config", func() {
			var resource1 db.Resource

			BeforeEach(func() {
				var err error
				var found bool
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
			})

			It("requests schedule on the jobs that use the resource", func() {
				job, found, err := pipeline.Job("job-using-resource")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				requestedSchedule := job.ScheduleRequestedTime()

				_, err = resource1.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
				Expect(err).NotTo(HaveOccurred())

				found, err = job.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(job.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
			})

			It("does not request schedule on the jobs that do not use the resource", func() {
				job, found, err := pipeline.Job("not-using-resource")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				requestedSchedule := job.ScheduleRequestedTime()

				_, err = resource1.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
				Expect(err).NotTo(HaveOccurred())

				found, err = job.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(job.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule))
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
				resourceScope         db.ResourceConfigScope
			)

			saveVersion := func(version atc.Version) {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "registry-image",
				}

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				resourceScope, err = resource.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = resourceScope.SaveVersions(nil, []atc.Version{version})
				Expect(err).ToNot(HaveOccurred())

				var found bool
				resourceConfigVersion, found, err = resourceScope.FindVersion(version)
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			}

			Context("when the version is exact matched", func() {
				BeforeEach(func() {
					saveVersion(atc.Version{"version": "12345"})
				})

				It("returns resource config version and true", func() {
					Expect(resourceConfigVersionFound).To(BeTrue())
					Expect(rcvID).To(Equal(resourceConfigVersion.ID()))
					Expect(foundErr).ToNot(HaveOccurred())
				})
			})

			Context("when the version is partially matched", func() {
				BeforeEach(func() {
					saveVersion(atc.Version{"version": "12345", "tag": "1.0.0"})
				})

				It("returns resource config version and true", func() {
					Expect(resourceConfigVersionFound).To(BeTrue())
					Expect(rcvID).To(Equal(resourceConfigVersion.ID()))
					Expect(foundErr).ToNot(HaveOccurred())
				})
			})

			Context("when there are multiple versions partially matched", func() {
				BeforeEach(func() {
					version1 := atc.Version{"version": "12345", "tag": "1.0.0"}
					saveVersion(version1)

					version2 := atc.Version{"version": "12345", "tag": "2.0.0"}
					saveVersion(version2)
				})

				It("returns resource config version with highest order and true", func() {
					Expect(resourceConfigVersionFound).To(BeTrue())
					Expect(rcvID).To(Equal(resourceConfigVersion.ID()))
					Expect(foundErr).ToNot(HaveOccurred())
				})
			})

			Context("when the check order is 0", func() {
				BeforeEach(func() {
					saveVersion(atc.Version{"version": "12345"})

					version = atc.Version{"version": "2"}
					created, err := resource.SaveUncheckedVersion(version, nil, resourceScope.ResourceConfig(), atc.VersionedResourceTypes{})
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

				_, err = brt.FindOrCreate(setupTx, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())
				_, err = resourceConfigFactory.FindOrCreateResourceConfig("registry-image", atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
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
			resourceScope        db.ResourceConfigScope
		)

		Context("with version filters", func() {
			var filter atc.Version
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

				resourceScope, err = resource.SetResourceConfig(atc.Source{"some": "other-repository"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				originalVersionSlice = []atc.Version{
					{"ref": "v0", "commit": "v0"},
					{"ref": "v1", "commit": "v1"},
					{"ref": "v2", "commit": "v2"},
				}

				err = resourceScope.SaveVersions(nil, originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				resourceVersions = make([]atc.ResourceVersion, 0)

				for i := 0; i < 3; i++ {
					rcv, found, err := resourceScope.FindVersion(atc.Version{
						"ref":    "v" + strconv.Itoa(i),
						"commit": "v" + strconv.Itoa(i),
					})
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

			Context("when filter include one field that matches", func() {
				BeforeEach(func() {
					filter = atc.Version{"ref": "v2"}
				})

				It("return version that matches field filter", func() {
					result, _, found, err := resource.Versions(db.Page{Limit: 10}, filter)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(result)).To(Equal(1))
					Expect(result[0].Version).To(Equal(resourceVersions[2].Version))
				})
			})

			Context("when filter include one field that doesn't match", func() {
				BeforeEach(func() {
					filter = atc.Version{"ref": "v20"}
				})

				It("return no version", func() {
					result, _, found, err := resource.Versions(db.Page{Limit: 10}, filter)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(result)).To(Equal(0))
				})
			})

			Context("when filter include two fields that match", func() {
				BeforeEach(func() {
					filter = atc.Version{"ref": "v1", "commit": "v1"}
				})

				It("return version", func() {
					result, _, found, err := resource.Versions(db.Page{Limit: 10}, filter)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(result)).To(Equal(1))
					Expect(result[0].Version).To(Equal(resourceVersions[1].Version))
				})
			})

			Context("when filter include two fields and one of them does not match", func() {
				BeforeEach(func() {
					filter = atc.Version{"ref": "v1", "commit": "v2"}
				})

				It("return no version", func() {
					result, _, found, err := resource.Versions(db.Page{Limit: 10}, filter)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(result)).To(Equal(0))
				})
			})
		})

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

				resourceScope, err = resource.SetResourceConfig(atc.Source{"some": "other-repository"}, atc.VersionedResourceTypes{})
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

				err = resourceScope.SaveVersions(nil, originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				resourceVersions = make([]atc.ResourceVersion, 0)

				for i := 0; i < 10; i++ {
					rcv, found, err := resourceScope.FindVersion(atc.Version{"ref": "v" + strconv.Itoa(i)})
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

				reloaded, err := resource.Reload()
				Expect(err).ToNot(HaveOccurred())
				Expect(reloaded).To(BeTrue())
			})

			Context("with no from/to", func() {
				It("returns the first page, with the given limit, and a next page", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{Limit: 2}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[8].Version))
					Expect(pagination.Newer).To(BeNil())
					Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(resourceVersions[7].ID), Limit: 2}))
				})
			})

			Context("with a to that places it in the middle of the versions", func() {
				It("returns the versions, with previous/next pages", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{To: db.NewIntPtr(resourceVersions[6].ID), Limit: 2}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[6].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[5].Version))
					Expect(pagination.Newer).To(Equal(&db.Page{From: db.NewIntPtr(resourceVersions[7].ID), Limit: 2}))
					Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(resourceVersions[4].ID), Limit: 2}))
				})
			})

			Context("with a to that places it to the oldest version", func() {
				It("returns the versions, with no next page", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{To: db.NewIntPtr(resourceVersions[1].ID), Limit: 2}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[1].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[0].Version))
					Expect(pagination.Newer).To(Equal(&db.Page{From: db.NewIntPtr(resourceVersions[2].ID), Limit: 2}))
					Expect(pagination.Older).To(BeNil())
				})
			})

			Context("with a from that places it in the middle of the versions", func() {
				It("returns the versions, with previous/next pages", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{From: db.NewIntPtr(resourceVersions[6].ID), Limit: 2}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[7].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[6].Version))
					Expect(pagination.Newer).To(Equal(&db.Page{From: db.NewIntPtr(resourceVersions[8].ID), Limit: 2}))
					Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(resourceVersions[5].ID), Limit: 2}))
				})
			})

			Context("with a from that places it at the beginning of the most recent versions", func() {
				It("returns the versions, with no previous page", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{From: db.NewIntPtr(resourceVersions[8].ID), Limit: 2}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[8].Version))
					Expect(pagination.Newer).To(BeNil())
					Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(resourceVersions[7].ID), Limit: 2}))
				})
			})

			Context("when the version has metadata", func() {
				BeforeEach(func() {
					metadata := []db.ResourceConfigMetadataField{{Name: "name1", Value: "value1"}}

					// save metadata
					_, err := resource.SaveUncheckedVersion(atc.Version(resourceVersions[9].Version), metadata, resourceScope.ResourceConfig(), atc.VersionedResourceTypes{})
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns the metadata in the version history", func() {
					historyPage, _, found, err := resource.Versions(db.Page{Limit: 1}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(1))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[0].Metadata).To(Equal([]atc.MetadataField{{Name: "name1", Value: "value1"}}))
				})

				It("maintains existing metadata after same version is saved with no metadata", func() {
					resourceScope, err := resource.SetResourceConfig(atc.Source{"some": "other-repository"}, atc.VersionedResourceTypes{})
					Expect(err).ToNot(HaveOccurred())

					err = resourceScope.SaveVersions(nil, []atc.Version{resourceVersions[9].Version})
					Expect(err).ToNot(HaveOccurred())

					historyPage, _, found, err := resource.Versions(db.Page{Limit: 1}, atc.Version{})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(1))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[0].Metadata).To(Equal([]atc.MetadataField{{Name: "name1", Value: "value1"}}))
				})

				It("updates metadata after same version is saved with different metadata", func() {
					newMetadata := []db.ResourceConfigMetadataField{{Name: "name-new", Value: "value-new"}}
					_, err := resource.SaveUncheckedVersion(atc.Version(resourceVersions[9].Version), newMetadata, resourceScope.ResourceConfig(), atc.VersionedResourceTypes{})

					historyPage, _, found, err := resource.Versions(db.Page{Limit: 1}, atc.Version{})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(1))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[0].Metadata).To(Equal([]atc.MetadataField{{Name: "name-new", Value: "value-new"}}))
				})
			})

			Context("when a version is disabled", func() {
				BeforeEach(func() {
					err := resource.DisableVersion(resourceVersions[9].ID)
					Expect(err).ToNot(HaveOccurred())

					resourceVersions[9].Enabled = false
				})

				It("returns a disabled version", func() {
					historyPage, _, found, err := resource.Versions(db.Page{Limit: 1}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(ConsistOf([]atc.ResourceVersion{resourceVersions[9]}))
				})
			})

			Context("when the version metadata is updated", func() {
				var metadata db.ResourceConfigMetadataFields

				BeforeEach(func() {
					metadata = []db.ResourceConfigMetadataField{{Name: "name1", Value: "value1"}}

					updated, err := resource.UpdateMetadata(resourceVersions[9].Version, metadata)
					Expect(err).ToNot(HaveOccurred())
					Expect(updated).To(BeTrue())
				})

				It("returns a version with metadata updated", func() {
					historyPage, _, found, err := resource.Versions(db.Page{Limit: 1}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(1))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[9].Version))
					Expect(historyPage[0].Metadata).To(Equal([]atc.MetadataField{{Name: "name1", Value: "value1"}}))
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

				resourceScope, err = resource.SetResourceConfig(atc.Source{"some": "other-repository"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				originalVersionSlice := []atc.Version{
					{"ref": "v1"}, // id: 1, check_order: 1
					{"ref": "v3"}, // id: 2, check_order: 2
					{"ref": "v4"}, // id: 3, check_order: 3
				}

				err = resourceScope.SaveVersions(nil, originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				secondVersionSlice := []atc.Version{
					{"ref": "v2"}, // id: 4, check_order: 4
					{"ref": "v3"}, // id: 2, check_order: 5
					{"ref": "v4"}, // id: 3, check_order: 6
				}

				err = resourceScope.SaveVersions(nil, secondVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				for i := 1; i < 5; i++ {
					rcv, found, err := resourceScope.FindVersion(atc.Version{"ref": "v" + strconv.Itoa(i)})
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

			Context("with no from/to", func() {
				It("returns versions ordered by check order", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{Limit: 4}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(4))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[3].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[2].Version))
					Expect(historyPage[2].Version).To(Equal(resourceVersions[1].Version))
					Expect(historyPage[3].Version).To(Equal(resourceVersions[0].Version))
					Expect(pagination.Newer).To(BeNil())
					Expect(pagination.Older).To(BeNil())
				})
			})

			Context("with from", func() {
				It("returns the versions, with previous/next pages including from", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{From: db.NewIntPtr(resourceVersions[1].ID), Limit: 2}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[2].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[1].Version))
					Expect(pagination.Newer).To(Equal(&db.Page{From: db.NewIntPtr(resourceVersions[3].ID), Limit: 2}))
					Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(resourceVersions[0].ID), Limit: 2}))
				})
			})

			Context("with to", func() {
				It("returns the builds, with previous/next pages including to", func() {
					historyPage, pagination, found, err := resource.Versions(db.Page{To: db.NewIntPtr(resourceVersions[2].ID), Limit: 2}, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage[0].Version).To(Equal(resourceVersions[2].Version))
					Expect(historyPage[1].Version).To(Equal(resourceVersions[1].Version))
					Expect(pagination.Newer).To(Equal(&db.Page{From: db.NewIntPtr(resourceVersions[3].ID), Limit: 2}))
					Expect(pagination.Older).To(Equal(&db.Page{To: db.NewIntPtr(resourceVersions[0].ID), Limit: 2}))
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

				resourceScope, err := resource.SetResourceConfig(atc.Source{"some": "other-repository"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				created, err := resource.SaveUncheckedVersion(atc.Version{"version": "not-returned"}, nil, resourceScope.ResourceConfig(), atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())
			})

			It("does not return the version", func() {
				historyPage, pagination, found, err := resource.Versions(db.Page{Limit: 2}, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(historyPage).To(BeNil())
				Expect(pagination).To(Equal(db.Pagination{Newer: nil, Older: nil}))
			})
		})
	})

	Describe("PinVersion/UnpinVersion", func() {
		var (
			resource      db.Resource
			resourceScope db.ResourceConfigScope
			resID         int
		)

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

			resourceScope, err = resource.SetResourceConfig(atc.Source{"some": "other-repository"}, atc.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			err = resourceScope.SaveVersions(nil, []atc.Version{
				atc.Version{"version": "v1"},
				atc.Version{"version": "v2"},
				atc.Version{"version": "v3"},
			})
			Expect(err).ToNot(HaveOccurred())

			resConf, found, err := resourceScope.FindVersion(atc.Version{"version": "v1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			resID = resConf.ID()
		})

		Context("when we use an invalid version id (does not exist)", func() {
			var (
				pinnedVersion atc.Version
			)
			BeforeEach(func() {
				found, err := resource.PinVersion(resID)
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				found, err = resource.Reload()
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				Expect(resource.CurrentPinnedVersion()).To(Equal(resource.APIPinnedVersion()))
				pinnedVersion = resource.APIPinnedVersion()
			})

			It("returns not found and does not update anything", func() {
				found, err := resource.PinVersion(-1)
				Expect(found).To(BeFalse())
				Expect(err).To(HaveOccurred())

				Expect(resource.APIPinnedVersion()).To(Equal(pinnedVersion))
			})
		})

		Context("when requesting schedule for version pinning", func() {
			It("requests schedule on all jobs using the resource", func() {
				job, found, err := pipeline.Job("job-using-resource")
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				requestedSchedule := job.ScheduleRequestedTime()

				found, err = resource.PinVersion(resID)
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				found, err = job.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(job.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
			})

			It("does not request schedule on jobs that do not use the resource", func() {
				job, found, err := pipeline.Job("not-using-resource")
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				requestedSchedule := job.ScheduleRequestedTime()

				found, err = resource.PinVersion(resID)
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				found, err = job.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(job.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule))
			})
		})

		Context("when we pin a resource to a version", func() {
			BeforeEach(func() {
				found, err := resource.PinVersion(resID)
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				found, err = resource.Reload()
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the resource is not pinned", func() {
				It("sets the api pinned version", func() {
					Expect(resource.APIPinnedVersion()).To(Equal(atc.Version{"version": "v1"}))
					Expect(resource.CurrentPinnedVersion()).To(Equal(resource.APIPinnedVersion()))
				})
			})

			Context("when the resource is pinned by another version already", func() {
				BeforeEach(func() {
					resConf, found, err := resourceScope.FindVersion(atc.Version{"version": "v3"})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					resID = resConf.ID()

					found, err = resource.PinVersion(resID)
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())

					found, err = resource.Reload()
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())
				})

				It("switch the pin to given version", func() {
					Expect(resource.APIPinnedVersion()).To(Equal(atc.Version{"version": "v3"}))
					Expect(resource.CurrentPinnedVersion()).To(Equal(resource.APIPinnedVersion()))
				})
			})

			Context("when we set the pin comment on a resource", func() {
				BeforeEach(func() {
					err := resource.SetPinComment("foo")
					Expect(err).ToNot(HaveOccurred())
					reload, err := resource.Reload()
					Expect(reload).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())
				})

				It("should set the pin comment", func() {
					Expect(resource.PinComment()).To(Equal("foo"))
				})
			})

			Context("when requesting schedule for version unpinning", func() {
				It("requests schedule on all jobs using the resource", func() {
					job, found, err := pipeline.Job("job-using-resource")
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())

					requestedSchedule := job.ScheduleRequestedTime()

					err = resource.UnpinVersion()
					Expect(err).ToNot(HaveOccurred())

					found, err = job.Reload()
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())

					Expect(job.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
				})

				It("does not request schedule on jobs that do not use the resource", func() {
					job, found, err := pipeline.Job("not-using-resource")
					Expect(found).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())

					requestedSchedule := job.ScheduleRequestedTime()

					err = resource.UnpinVersion()
					Expect(err).ToNot(HaveOccurred())

					Expect(job.ScheduleRequestedTime()).Should(BeTemporally("==", requestedSchedule))
				})
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

		Context("when we pin a resource that is already pinned to a version (through the config)", func() {
			var resConf db.ResourceConfigVersion

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

				resourceScope, err := resource.SetResourceConfig(atc.Source{"some": "repository"}, atc.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				err = resourceScope.SaveVersions(nil, []atc.Version{
					atc.Version{"version": "v1"},
					atc.Version{"version": "v2"},
					atc.Version{"version": "v3"},
				})
				Expect(err).ToNot(HaveOccurred())

				resConf, found, err = resourceScope.FindVersion(atc.Version{"version": "v1"})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("should fail to update the pinned version", func() {
				found, err := resource.PinVersion(resConf.ID())
				Expect(found).To(BeFalse())
				Expect(err).To(Equal(db.ErrPinnedThroughConfig))
			})
		})
	})

	Describe("Public", func() {
		var (
			resource db.Resource
			found    bool
			err      error
		)

		Context("when public is not set in the config", func() {
			BeforeEach(func() {
				resource, found, err = pipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("returns false", func() {
				Expect(resource.Public()).To(BeFalse())
			})
		})

		Context("when public is set to true in the config", func() {
			BeforeEach(func() {
				resource, found, err = pipeline.Resource("some-other-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("returns true", func() {
				Expect(resource.Public()).To(BeTrue())
			})
		})

		Context("when public is set to false in the config", func() {
			BeforeEach(func() {
				resource, found, err = pipeline.Resource("some-secret-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})

			It("returns false", func() {
				Expect(resource.Public()).To(BeFalse())
			})
		})
	})
})
