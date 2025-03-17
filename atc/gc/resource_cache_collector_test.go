package gc_test

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"
	"github.com/concourse/concourse/atc/gc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceCacheCollector", func() {
	var collector GcCollector
	var buildCollector GcCollector

	BeforeEach(func() {
		collector = gc.NewResourceCacheCollector(resourceCacheLifecycle)
		buildCollector = gc.NewBuildCollector(buildFactory)
	})

	Describe("Run", func() {
		Describe("resource caches", func() {
			var resourceCacheUseCollector GcCollector

			var oneOffBuild db.Build
			var jobBuild db.Build

			var oneOffCache db.ResourceCache
			var jobCache db.ResourceCache

			var scenario *dbtest.Scenario

			BeforeEach(func() {
				resourceCacheUseCollector = gc.NewResourceCacheUseCollector(resourceCacheLifecycle)

				scenario = dbtest.Setup(
					builder.WithPipeline(atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name:   "some-resource",
								Type:   "some-base-type",
								Source: atc.Source{"some": "source"},
							},
						},
						Jobs: atc.JobConfigs{
							{
								Name: "some-job",
							},
							{
								Name: "some-other-job",
							},
						},
					}),
					builder.WithResourceVersions("some-resource"),
				)

				oneOffBuild, err = scenario.Team.CreateOneOffBuild()
				Expect(err).ToNot(HaveOccurred())

				oneOffCache, err = resourceCacheFactory.FindOrCreateResourceCache(
					db.ForBuild(oneOffBuild.ID()),
					"some-base-type",
					atc.Version{"some": "version"},
					atc.Source{
						"some": "source",
					},
					nil,
					nil,
				)
				Expect(err).NotTo(HaveOccurred())

				jobBuild, err = scenario.Job("some-job").CreateBuild("someone")
				Expect(err).ToNot(HaveOccurred())

				jobCache, err = resourceCacheFactory.FindOrCreateResourceCache(
					db.ForBuild(jobBuild.ID()),
					"some-base-type",
					atc.Version{"some": "version"},
					atc.Source{
						"some": "source",
					},
					nil,
					nil,
				)
				Expect(err).NotTo(HaveOccurred())
			})

			resourceCacheExists := func(resourceCache db.ResourceCache) bool {
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
						var versionMD5 string
						version := `{"some":"version"}`
						err = psql.Insert("resource_config_versions").
							Columns("version", "version_md5", "metadata", "resource_config_scope_id").
							Values(version, sq.Expr(fmt.Sprintf("md5('%s')", version)), `null`, jobCache.ResourceConfig().ID()).
							Suffix("RETURNING version_md5").
							RunWith(dbConn).QueryRow().Scan(&versionMD5)
						Expect(err).NotTo(HaveOccurred())

						Expect(scenario.Job("some-job").SaveNextInputMapping(db.InputMapping{
							"whatever": db.InputResult{
								Input: &db.AlgorithmInput{
									AlgorithmVersion: db.AlgorithmVersion{
										Version:    db.ResourceVersion(versionMD5),
										ResourceID: scenario.Resource("some-resource").ID(),
									},
								},
							},
						}, true)).To(Succeed())
					})

					Context("when pipeline is paused", func() {
						BeforeEach(func() {
							err := scenario.Pipeline.Pause("")
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
						var secondJobCache db.ResourceCache

						BeforeEach(func() {
							secondJobBuild, err = scenario.Job("some-job").CreateBuild("someone")
							Expect(err).ToNot(HaveOccurred())

							secondJobCache, err = resourceCacheFactory.FindOrCreateResourceCache(
								db.ForBuild(secondJobBuild.ID()),
								"some-base-type",
								atc.Version{"some": "new-version"},
								atc.Source{
									"some": "source",
								},
								nil,
								nil,
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
						var secondJobCache db.ResourceCache

						BeforeEach(func() {
							secondJobBuild, err = scenario.Job("some-other-job").CreateBuild("someone")
							Expect(err).ToNot(HaveOccurred())

							secondJobCache, err = resourceCacheFactory.FindOrCreateResourceCache(
								db.ForBuild(secondJobBuild.ID()),
								"some-base-type",
								atc.Version{"some": "new-version"},
								atc.Source{
									"some": "source",
								},
								nil,
								nil,
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
