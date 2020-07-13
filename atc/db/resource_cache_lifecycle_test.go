package db_test

import (
	"fmt"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"

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
			resourceCacheForOneOffBuild := func() (db.UsedResourceCache, db.Build) {
				build, err := defaultTeam.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())
				return createResourceCacheWithUser(db.ForBuild(build.ID())), build
			}

			resourceCacheForJobBuild := func() (db.UsedResourceCache, db.Build) {
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
					setBuildStatus := func(a db.BuildStatus) (db.UsedResourceCache, db.Build) {
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
							Expect(resourceCache1.ID()).ToNot(Equal(resourceCache2.ID()))

							Expect(countResourceCaches()).To(Equal(2))

							err := resourceCacheLifecycle.CleanUpInvalidCaches(logger.Session("resource-cache-lifecycle"))
							Expect(err).ToNot(HaveOccurred())

							Expect(countResourceCaches()).To(Equal(1))
						})
						It("does not remove the resource caches from other jobs", func() {
							By("creating a second pipeline")
							secondPipeline, _, err := defaultTeam.SavePipeline(atc.PipelineRef{Name: "second-pipeline"}, atc.Config{
								Jobs: atc.JobConfigs{
									{
										Name: "some-job",
									},
								},
								Resources: atc.ResourceConfigs{
									{
										Name: "some-resource",
										Type: "some-base-resource-type",
										Source: atc.Source{
											"some": "source",
										},
									},
								},
								ResourceTypes: atc.ResourceTypes{
									{
										Name: "some-type",
										Type: "some-base-resource-type",
										Source: atc.Source{
											"some-type": "source",
										},
									},
								},
							}, db.ConfigVersion(0), false)
							Expect(err).NotTo(HaveOccurred())

							By("creating an image resource cache tied to the job in the second pipeline")
							job, _, err := secondPipeline.Job("some-job")
							Expect(err).ToNot(HaveOccurred())
							build, err := job.CreateBuild()
							Expect(err).ToNot(HaveOccurred())
							resourceCache := createResourceCacheWithUser(db.ForBuild(build.ID()))

							err = build.SaveImageResourceVersion(resourceCache)
							Expect(err).ToNot(HaveOccurred())

							err = build.SetInterceptible(false)
							Expect(err).ToNot(HaveOccurred())

							By("creating an image resource cached in the default pipeline")
							setBuildStatus(db.BuildStatusSucceeded)

							Expect(countResourceCaches()).To(Equal(2))
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
				resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(
					"some-base-resource-type",
					atc.Source{
						"some": "source",
					},
					atc.VersionedResourceTypes{},
				)
				Expect(err).ToNot(HaveOccurred())

				containerOwner = db.NewResourceConfigCheckSessionContainerOwner(
					resourceConfig.ID(),
					resourceConfig.OriginBaseResourceType().ID,
					db.ContainerOwnerExpiries{},
				)

				container, err = defaultWorker.CreateContainer(containerOwner, db.ContainerMetadata{})
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
				_, err := resourceConfigFactory.FindOrCreateResourceConfig(
					"some-type",
					atc.Source{
						"some": "source",
					},
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
				)
				Expect(err).ToNot(HaveOccurred())

				Expect(countResourceCaches()).ToNot(BeZero())
				err = resourceCacheLifecycle.CleanUpInvalidCaches(logger.Session("resource-cache-lifecycle"))
				Expect(err).ToNot(HaveOccurred())

				Expect(countResourceCaches()).ToNot(BeZero())
			})

			It("removes the cache if the type is no longer configured", func() {
				_, err := resourceConfigFactory.FindOrCreateResourceConfig(
					"some-type",
					atc.Source{
						"some": "source",
					},
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

				resourceConfigScope, err := defaultResource.SetResourceConfig(
					atc.Source{"some": "source"},
					atc.VersionedResourceTypes{},
				)
				Expect(err).ToNot(HaveOccurred())

				containerOwner := db.NewResourceConfigCheckSessionContainerOwner(
					resourceConfigScope.ResourceConfig().ID(),
					resourceConfigScope.ResourceConfig().OriginBaseResourceType().ID,
					db.ContainerOwnerExpiries{},
				)

				container, err := defaultWorker.CreateContainer(containerOwner, db.ContainerMetadata{})
				Expect(err).ToNot(HaveOccurred())

				_ = createResourceCacheWithUser(db.ForContainer(container.ID()))

				err = resourceConfigScope.SaveVersions(nil, []atc.Version{{"some": "version"}})
				Expect(err).ToNot(HaveOccurred())

				resourceConfigVersion, found, err := resourceConfigScope.FindVersion(atc.Version{"some": "version"})
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				err = defaultJob.SaveNextInputMapping(db.InputMapping{
					"some-resource": db.InputResult{
						Input: &db.AlgorithmInput{
							AlgorithmVersion: db.AlgorithmVersion{
								Version:    db.ResourceVersion(convertToMD5(atc.Version(resourceConfigVersion.Version()))),
								ResourceID: defaultResource.ID(),
							},
						},
						PassedBuildIDs: []int{},
					},
				}, true)
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

func createResourceCacheWithUser(resourceCacheUser db.ResourceCacheUser) db.UsedResourceCache {
	usedResourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
		resourceCacheUser,
		"some-base-resource-type",
		atc.Version{"some": "version"},
		atc.Source{
			"some": "source",
		},
		atc.Params{"some": fmt.Sprintf("param-%d", time.Now().UnixNano())},
		atc.VersionedResourceTypes{},
	)
	Expect(err).ToNot(HaveOccurred())

	return usedResourceCache
}
