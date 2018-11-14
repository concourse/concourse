package gc_test

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/algorithm"
	"github.com/concourse/concourse/atc/gc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceCacheCollector", func() {
	var collector gc.Collector
	var buildCollector gc.Collector

	BeforeEach(func() {
		collector = gc.NewResourceCacheCollector(resourceCacheLifecycle)
		buildCollector = gc.NewBuildCollector(buildFactory)
	})

	Describe("Run", func() {
		Describe("resource caches", func() {
			var resourceCacheUseCollector gc.Collector

			var oneOffBuild db.Build
			var jobBuild db.Build

			var oneOffCache db.UsedResourceCache
			var jobCache db.UsedResourceCache

			var resource db.Resource

			BeforeEach(func() {
				resourceCacheUseCollector = gc.NewResourceCacheUseCollector(resourceCacheLifecycle)

				oneOffBuild, err = defaultTeam.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())

				oneOffCache, err = resourceCacheFactory.FindOrCreateResourceCache(
					logger,
					db.ForBuild(oneOffBuild.ID()),
					"some-base-type",
					atc.Version{"some": "version"},
					atc.Source{
						"some": "source",
					},
					nil,
					creds.VersionedResourceTypes{},
				)
				Expect(err).NotTo(HaveOccurred())

				jobBuild, err = defaultJob.CreateBuild()
				Expect(err).ToNot(HaveOccurred())

				jobCache, err = resourceCacheFactory.FindOrCreateResourceCache(
					logger,
					db.ForBuild(jobBuild.ID()),
					"some-base-type",
					atc.Version{"some": "version"},
					atc.Source{
						"some": "source",
					},
					nil,
					creds.VersionedResourceTypes{},
				)
				Expect(err).NotTo(HaveOccurred())

				var found bool
				resource, found, err = defaultPipeline.Resource("some-resource")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				err = resource.SetResourceConfig(jobCache.ResourceConfig().ID())
				Expect(err).ToNot(HaveOccurred())
			})

			resourceCacheExists := func(resourceCache db.UsedResourceCache) bool {
				var count int
				err = psql.Select("COUNT(*)").
					From("resource_caches").
					Where(sq.Eq{"id": resourceCache.ID()}).
					RunWith(dbConn).
					QueryRow().
					Scan(&count)
				Expect(err).NotTo(HaveOccurred())
				return count == 1
			}

			JustBeforeEach(func() {
				Expect(buildCollector.Run(context.TODO())).To(Succeed())
				Expect(resourceCacheUseCollector.Run(context.TODO())).To(Succeed())
				Expect(collector.Run(context.TODO())).To(Succeed())
			})

			Context("when the resource cache is still in use", func() {
				It("does not delete the cache", func() {
					Expect(collector.Run(context.TODO())).To(Succeed())
					Expect(resourceCacheExists(oneOffCache)).To(BeTrue())
					Expect(resourceCacheExists(jobCache)).To(BeTrue())
				})
			})

			Context("when the cache is no longer in use", func() {
				BeforeEach(func() {
					Expect(oneOffBuild.Finish(db.BuildStatusSucceeded)).To(Succeed())
					Expect(jobBuild.Finish(db.BuildStatusSucceeded)).To(Succeed())
				})

				Context("when the cache is an input to a job", func() {
					BeforeEach(func() {
						var versionID int
						version := `{"some":"version"}`
						err = psql.Insert("resource_config_versions").
							Columns("version", "version_md5", "metadata", "resource_config_id").
							Values(version, sq.Expr(fmt.Sprintf("md5('%s')", version)), `null`, jobCache.ResourceConfig().ID()).
							Suffix("RETURNING id").
							RunWith(dbConn).QueryRow().Scan(&versionID)
						Expect(err).NotTo(HaveOccurred())

						Expect(defaultJob.SaveNextInputMapping(algorithm.InputMapping{
							"whatever": algorithm.InputVersion{
								VersionID:  versionID,
								ResourceID: resource.ID(),
							},
						})).To(Succeed())
					})

					Context("when pipeline is paused", func() {
						BeforeEach(func() {
							err := defaultPipeline.Pause()
							Expect(err).NotTo(HaveOccurred())
						})

						It("removes the cache", func() {
							Expect(resourceCacheExists(jobCache)).To(BeFalse())
						})
					})

					Context("when pipeline is not paused", func() {
						It("leaves it alone", func() {
							Expect(resourceCacheExists(jobCache)).To(BeTrue())
						})
					})
				})

				Context("when the cache is an image resource version for a job build", func() {
					BeforeEach(func() {
						err := jobBuild.SaveImageResourceVersion(jobCache)
						Expect(err).NotTo(HaveOccurred())
					})

					It("leaves it alone", func() {
						Expect(resourceCacheExists(jobCache)).To(BeTrue())
					})

					Context("when another build of the same job exists with a different image cache", func() {
						var secondJobBuild db.Build
						var secondJobCache db.UsedResourceCache

						BeforeEach(func() {
							secondJobBuild, err = defaultJob.CreateBuild()
							Expect(err).ToNot(HaveOccurred())

							secondJobCache, err = resourceCacheFactory.FindOrCreateResourceCache(
								logger,
								db.ForBuild(secondJobBuild.ID()),
								"some-base-type",
								atc.Version{"some": "new-version"},
								atc.Source{
									"some": "source",
								},
								nil,
								creds.VersionedResourceTypes{},
							)
							Expect(err).NotTo(HaveOccurred())

							Expect(secondJobBuild.SaveImageResourceVersion(secondJobCache)).To(Succeed())
						})

						Context("when the second build succeeds", func() {
							BeforeEach(func() {
								Expect(secondJobBuild.Finish(db.BuildStatusSucceeded)).To(Succeed())
							})

							It("keeps the new cache and removes the old one", func() {
								Expect(resourceCacheExists(jobCache)).To(BeFalse())
								Expect(resourceCacheExists(secondJobCache)).To(BeTrue())
							})
						})

						Context("when the second build fails", func() {
							BeforeEach(func() {
								Expect(secondJobBuild.Finish(db.BuildStatusFailed)).To(Succeed())
							})

							It("keeps the new cache and the old one", func() {
								Expect(resourceCacheExists(jobCache)).To(BeTrue())
								Expect(resourceCacheExists(secondJobCache)).To(BeTrue())
							})
						})
					})

					Context("when another build of a different job exists with a different image cache", func() {
						var secondJobBuild db.Build
						var secondJobCache db.UsedResourceCache

						BeforeEach(func() {
							secondJob, found, err := defaultPipeline.Job("some-other-job")
							Expect(err).NotTo(HaveOccurred())
							Expect(found).To(BeTrue())

							secondJobBuild, err = secondJob.CreateBuild()
							Expect(err).ToNot(HaveOccurred())

							secondJobCache, err = resourceCacheFactory.FindOrCreateResourceCache(
								logger,
								db.ForBuild(secondJobBuild.ID()),
								"some-base-type",
								atc.Version{"some": "new-version"},
								atc.Source{
									"some": "source",
								},
								nil,
								creds.VersionedResourceTypes{},
							)
							Expect(err).NotTo(HaveOccurred())

							Expect(secondJobBuild.SaveImageResourceVersion(secondJobCache)).To(Succeed())
						})

						Context("when the second build succeeds", func() {
							BeforeEach(func() {
								Expect(secondJobBuild.Finish(db.BuildStatusSucceeded)).To(Succeed())
							})

							It("keeps the new cache and the old one", func() {
								Expect(resourceCacheExists(jobCache)).To(BeTrue())
								Expect(resourceCacheExists(secondJobCache)).To(BeTrue())
							})
						})

						Context("when the second build fails", func() {
							BeforeEach(func() {
								Expect(secondJobBuild.Finish(db.BuildStatusFailed)).To(Succeed())
							})

							It("keeps the new cache and the old one", func() {
								Expect(resourceCacheExists(jobCache)).To(BeTrue())
								Expect(resourceCacheExists(secondJobCache)).To(BeTrue())
							})
						})
					})
				})

				Context("when the cache is an image resource version for a one-off build", func() {
					BeforeEach(func() {
						err := oneOffBuild.SaveImageResourceVersion(oneOffCache)
						Expect(err).NotTo(HaveOccurred())
					})

					Context("when the build finished recently", func() {
						It("leaves it alone", func() {
							Expect(resourceCacheExists(oneOffCache)).To(BeTrue())
						})
					})

					Context("when the build finished a day ago", func() {
						BeforeEach(func() {
							_, err := psql.Update("builds").
								Set("end_time", sq.Expr("NOW() - '25 hours'::interval")).
								Where(sq.Eq{"id": oneOffBuild.ID()}).
								RunWith(dbConn).
								Exec()
							Expect(err).ToNot(HaveOccurred())
						})

						It("removes the cache", func() {
							Expect(resourceCacheExists(oneOffCache)).To(BeFalse())
						})
					})
				})
			})
		})
	})
})
