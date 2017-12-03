package db_test

import (
	"fmt"
	"time"

	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceCacheLifecycle", func() {

	var resourceCacheLifecycle db.ResourceCacheLifecycle

	BeforeEach(func() {
		resourceCacheLifecycle = db.NewResourceCacheLifecycle(dbConn)
	})

	Describe("CleanUpInvalidCaches", func() {
		Context("the resource cache is used by a build", func() {
			resourceCacheForOneOffBuild := func() (*db.UsedResourceCache, db.Build) {
				build, err := defaultTeam.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())
				return createResourceCacheWithUser(db.ForBuild(build.ID())), build
			}

			resourceCacheForJobBuild := func() (*db.UsedResourceCache, db.Build) {
				build, err := defaultJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())
				return createResourceCacheWithUser(db.ForBuild(build.ID())), build
			}

			Context("when its a one off build", func() {
				It("doesn't delete the resource cache", func() {
					_, _ = resourceCacheForOneOffBuild()

					err := resourceCacheLifecycle.CleanUpInvalidCaches(logger.Session("resource-cache-lifecycle"))
					Expect(err).ToNot(HaveOccurred())
					Expect(countResourceCaches()).ToNot(BeZero())
				})

				Context("when the build goes away", func() {
					BeforeEach(func() {
						_, build := resourceCacheForOneOffBuild()

						_, err := build.Delete()
						Expect(err).ToNot(HaveOccurred())

						Expect(countResourceCaches()).ToNot(BeZero())

						err = resourceCacheLifecycle.CleanUpInvalidCaches(logger.Session("resource-cache-lifecycle"))
						Expect(err).ToNot(HaveOccurred())
					})

					It("deletes the resource cache", func() {
						Expect(countResourceCaches()).To(BeZero())
					})
				})

				Context("when the cache is for a saved image resource version for a finished build", func() {

					setBuildStatus := func(a db.BuildStatus) {
						resourceCache, build := resourceCacheForOneOffBuild()

						err := build.SaveImageResourceVersion(resourceCache)
						Expect(err).ToNot(HaveOccurred())

						err = build.SetInterceptible(false)
						Expect(err).ToNot(HaveOccurred())

						err = build.Finish(a)
						Expect(err).ToNot(HaveOccurred())

						err = resourceCacheLifecycle.CleanUsesForFinishedBuilds(logger)
						Expect(err).ToNot(HaveOccurred())
					}

					Context("when the build has succeeded", func() {
						It("does not remove the image resource cache", func() {
							setBuildStatus(db.BuildStatusSucceeded)
							Expect(countResourceCaches()).ToNot(BeZero())

							err := resourceCacheLifecycle.CleanUpInvalidCaches(logger.Session("resource-cache-lifecycle"))
							Expect(err).ToNot(HaveOccurred())

							Expect(countResourceCaches()).ToNot(BeZero())
						})
					})

					Context("when build has not succeeded", func() {
						It("does not removes the image resource cache", func() {
							setBuildStatus(db.BuildStatusFailed)
							Expect(countResourceCaches()).ToNot(BeZero())

							err := resourceCacheLifecycle.CleanUpInvalidCaches(logger.Session("resource-cache-lifecycle"))
							Expect(err).ToNot(HaveOccurred())

							Expect(countResourceCaches()).ToNot(BeZero())
						})
					})
				})
			})

			Context("when its a build of a job in a pipeline", func() {
				Context("when the cache is for a saved image resource version for a finished build", func() {
					setBuildStatus := func(a db.BuildStatus) (*db.UsedResourceCache, db.Build) {
						resourceCache, build := resourceCacheForJobBuild()
						Expect(build.JobID()).ToNot(BeZero())

						err := build.SaveImageResourceVersion(resourceCache)
						Expect(err).ToNot(HaveOccurred())

						err = build.SetInterceptible(false)
						Expect(err).ToNot(HaveOccurred())

						err = build.Finish(a)
						Expect(err).ToNot(HaveOccurred())

						err = resourceCacheLifecycle.CleanUsesForFinishedBuilds(logger)
						Expect(err).ToNot(HaveOccurred())
						return resourceCache, build
					}

					Context("when the build has succeeded", func() {
						It("does not remove the resource cache for the most recent build", func() {

							setBuildStatus(db.BuildStatusSucceeded)
							Expect(countResourceCaches()).To(Equal(1))

							err := resourceCacheLifecycle.CleanUpInvalidCaches(logger.Session("resource-cache-lifecycle"))
							Expect(err).ToNot(HaveOccurred())

							Expect(countResourceCaches()).To(Equal(1))
						})

						It("removes resource caches for previous finished builds", func() {
							resourceCache1, _ := setBuildStatus(db.BuildStatusFailed)
							resourceCache2, _ := setBuildStatus(db.BuildStatusSucceeded)
							Expect(resourceCache1.ID).ToNot(Equal(resourceCache2.ID))

							Expect(countResourceCaches()).To(Equal(2))

							err := resourceCacheLifecycle.CleanUpInvalidCaches(logger.Session("resource-cache-lifecycle"))
							Expect(err).ToNot(HaveOccurred())

							Expect(countResourceCaches()).To(Equal(1))
						})
					})

					Context("when build has not succeeded", func() {
						It("does not remove the image resource cache", func() {
							setBuildStatus(db.BuildStatusFailed)
							Expect(countResourceCaches()).ToNot(BeZero())

							err := resourceCacheLifecycle.CleanUpInvalidCaches(logger.Session("resource-cache-lifecycle"))
							Expect(err).ToNot(HaveOccurred())

							Expect(countResourceCaches()).ToNot(BeZero())
						})
					})
				})
			})
		})

		Context("the resource cache is used by a container", func() {
			var (
				container      db.CreatingContainer
				containerOwner db.ContainerOwner
			)

			BeforeEach(func() {
				var err error

				resourceConfigCheckSession, err := resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(
					logger,
					"some-base-resource-type",
					atc.Source{
						"some": "source",
					},
					creds.NewVersionedResourceTypes(
						template.StaticVariables{"source-param": "some-secret-sauce"},
						atc.VersionedResourceTypes{},
					),
					db.ContainerOwnerExpiries{},
				)

				Expect(err).ToNot(HaveOccurred())

				containerOwner = db.NewResourceConfigCheckSessionContainerOwner(resourceConfigCheckSession, defaultTeam.ID())

				container, err = defaultTeam.CreateContainer(defaultWorker.Name(), containerOwner, db.ContainerMetadata{})
				Expect(err).ToNot(HaveOccurred())

				_ = createResourceCacheWithUser(db.ForContainer(container.ID()))
			})

			Context("and the container still exists", func() {
				BeforeEach(func() {
					err := resourceCacheLifecycle.CleanUpInvalidCaches(logger.Session("resource-cache-lifecycle"))
					Expect(err).ToNot(HaveOccurred())
				})

				It("doesn't delete the resource cache", func() {
					Expect(countResourceCaches()).ToNot(BeZero())
				})
			})

			Context("and the container has been deleted", func() {
				BeforeEach(func() {
					createdContainer, err := container.Created()
					Expect(err).ToNot(HaveOccurred())

					destroyingContainer, err := createdContainer.Destroying()
					Expect(err).ToNot(HaveOccurred())

					_, err = destroyingContainer.Destroy()
					Expect(err).ToNot(HaveOccurred())

					err = resourceCacheLifecycle.CleanUpInvalidCaches(logger.Session("resource-cache-lifecycle"))
					Expect(err).ToNot(HaveOccurred())
				})

				It("deletes the resource cache", func() {
					Expect(countResourceCaches()).To(BeZero())
				})
			})
		})

		Context("when the cache is for a custom resource type", func() {
			It("does not remove the cache if the type is still configured", func() {
				_, err := resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(
					logger,
					"some-type",
					atc.Source{
						"some": "source",
					},
					creds.NewVersionedResourceTypes(
						template.StaticVariables{"source-param": "some-secret-sauce"},
						atc.VersionedResourceTypes{
							atc.VersionedResourceType{
								ResourceType: atc.ResourceType{
									Name: "some-type",
									Type: "some-base-resource-type",
									Source: atc.Source{
										"some": "source",
									},
								},
								Version: atc.Version{"showme": "whatyougot"},
							},
						},
					),
					db.ContainerOwnerExpiries{},
				)
				Expect(err).ToNot(HaveOccurred())

				Expect(countResourceCaches()).ToNot(BeZero())
				err = resourceCacheLifecycle.CleanUpInvalidCaches(logger.Session("resource-cache-lifecycle"))
				Expect(err).ToNot(HaveOccurred())

				Expect(countResourceCaches()).ToNot(BeZero())
			})

			It("removes the cache if the type is no longer configured", func() {
				_, err := resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(
					logger,
					"some-type",
					atc.Source{
						"some": "source",
					},
					creds.NewVersionedResourceTypes(
						template.StaticVariables{"source-param": "some-secret-sauce"},
						atc.VersionedResourceTypes{
							atc.VersionedResourceType{
								ResourceType: atc.ResourceType{
									Name: "some-type",
									Type: "some-base-resource-type",
									Source: atc.Source{
										"some": "source",
									},
								},
								Version: atc.Version{"showme": "whatyougot"},
							},
						},
					),
					db.ContainerOwnerExpiries{},
				)
				Expect(err).ToNot(HaveOccurred())

				err = resourceConfigCheckSessionLifecycle.CleanInactiveResourceConfigCheckSessions()
				Expect(err).ToNot(HaveOccurred())

				err = resourceConfigFactory.CleanUnreferencedConfigs()
				Expect(err).ToNot(HaveOccurred())

				Expect(countResourceCaches()).ToNot(BeZero())

				err = resourceCacheLifecycle.CleanUpInvalidCaches(logger.Session("resource-cache-lifecycle"))
				Expect(err).ToNot(HaveOccurred())

				Expect(countResourceCaches()).To(BeZero())
			})
		})

		Context("when the cache is for a resource version used as an input for the next build of a job", func() {
			It("does not remove the cache", func() {
				err := defaultPipeline.Unpause()
				Expect(err).ToNot(HaveOccurred())

				resourceConfigCheckSession, err := resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(
					logger,
					"some-base-resource-type",
					atc.Source{
						"some": "source",
					},
					creds.NewVersionedResourceTypes(
						template.StaticVariables{"source-param": "some-secret-sauce"},
						atc.VersionedResourceTypes{},
					),
					db.ContainerOwnerExpiries{},
				)
				Expect(err).ToNot(HaveOccurred())

				containerOwner := db.NewResourceConfigCheckSessionContainerOwner(resourceConfigCheckSession, defaultTeam.ID())

				container, err := defaultTeam.CreateContainer(defaultWorker.Name(), containerOwner, db.ContainerMetadata{})
				Expect(err).ToNot(HaveOccurred())

				rc := createResourceCacheWithUser(db.ForContainer(container.ID()))

				err = defaultResource.SetResourceConfig(rc.ResourceConfig.ID)
				Expect(err).ToNot(HaveOccurred())

				err = defaultPipeline.SaveResourceVersions(atc.ResourceConfig{
					Name: defaultResource.Name(),
					Type: defaultResource.Type(),
				}, []atc.Version{{"some": "version"}})
				Expect(err).ToNot(HaveOccurred())

				versionedResource, found, err := defaultPipeline.GetVersionedResourceByVersion(atc.Version{"some": "version"}, defaultResource.Name())
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				err = defaultJob.SaveNextInputMapping(algorithm.InputMapping{
					"some-resource": algorithm.InputVersion{
						VersionID: versionedResource.ID,
					},
				})
				Expect(err).ToNot(HaveOccurred())

				createdContainer, err := container.Created()
				Expect(err).ToNot(HaveOccurred())

				destroyingContainer, err := createdContainer.Destroying()
				Expect(err).ToNot(HaveOccurred())

				_, err = destroyingContainer.Destroy()
				Expect(err).ToNot(HaveOccurred())

				Expect(countResourceCaches()).ToNot(BeZero())

				err = resourceCacheLifecycle.CleanUpInvalidCaches(logger.Session("resource-cache-lifecycle"))
				Expect(err).ToNot(HaveOccurred())

				Expect(countResourceCaches()).ToNot(BeZero())
			})
		})
	})
})

func countResourceCaches() int {
	var result int
	err := psql.Select("count(*)").
		From("resource_caches").
		RunWith(dbConn).
		QueryRow().
		Scan(&result)
	Expect(err).ToNot(HaveOccurred())

	return result

}

func createResourceCacheWithUser(resourceCacheUser db.ResourceCacheUser) *db.UsedResourceCache {
	usedResourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
		logger,
		resourceCacheUser,
		"some-base-resource-type",
		atc.Version{"some": "version"},
		atc.Source{
			"some": fmt.Sprintf("param-%d", time.Now().UnixNano()),
		},
		atc.Params{"some": fmt.Sprintf("param-%d", time.Now().UnixNano())},
		creds.NewVersionedResourceTypes(
			template.StaticVariables{"source-param": "some-secret-sauce"},
			atc.VersionedResourceTypes{},
		),
	)
	Expect(err).ToNot(HaveOccurred())

	return usedResourceCache
}
