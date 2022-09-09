package gc_test

import (
	"context"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/gc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceCacheUseCollector", func() {
	var collector GcCollector
	var buildCollector GcCollector

	BeforeEach(func() {
		collector = gc.NewResourceCacheUseCollector(resourceCacheLifecycle)
		buildCollector = gc.NewBuildCollector(buildFactory)
	})

	Describe("Run", func() {
		Describe("cache uses", func() {
			var (
				customResourceTypeCache db.ResourceCache
			)

			countResourceCacheUses := func() int {
				tx, err := dbConn.Begin()
				Expect(err).NotTo(HaveOccurred())
				defer func() {
					_ = tx.Rollback()
				}()

				var result int
				err = psql.Select("count(*)").
					From("resource_cache_uses").
					RunWith(tx).
					QueryRow().
					Scan(&result)
				Expect(err).NotTo(HaveOccurred())

				return result
			}

			Describe("for one-off builds", func() {
				BeforeEach(func() {
					var err error
					customResourceTypeCache, err = resourceCacheFactory.FindOrCreateResourceCache(
						db.ForBuild(defaultBuild.ID()),
						"some-base-type",
						atc.Version{"some-type": "version"},
						atc.Source{
							"some-type": "source-param",
						},
						nil,
						nil,
					)
					Expect(err).NotTo(HaveOccurred())

					_, err = resourceCacheFactory.FindOrCreateResourceCache(
						db.ForBuild(defaultBuild.ID()),
						"some-type",
						atc.Version{"some": "version"},
						atc.Source{
							"some": "source",
						},
						atc.Params{"some": "params"},
						customResourceTypeCache,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("before the build has completed", func() {
					It("does not clean up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						Expect(collector.Run(context.TODO())).To(Succeed())
						Expect(countResourceCacheUses()).NotTo(BeZero())
					})
				})

				Context("once the build has completed successfully", func() {
					It("cleans up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						Expect(defaultBuild.Finish(db.BuildStatusSucceeded)).To(Succeed())
						Expect(buildCollector.Run(context.TODO())).To(Succeed())
						Expect(collector.Run(context.TODO())).To(Succeed())
						Expect(countResourceCacheUses()).To(BeZero())
					})
				})

				Context("once the build has been aborted", func() {
					It("cleans up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						Expect(defaultBuild.Finish(db.BuildStatusAborted)).To(Succeed())
						Expect(buildCollector.Run(context.TODO())).To(Succeed())
						Expect(collector.Run(context.TODO())).To(Succeed())
						Expect(countResourceCacheUses()).To(BeZero())
					})
				})

				Context("once the build has failed", func() {
					Context("when the build is a one-off", func() {
						It("cleans up the uses", func() {
							Expect(countResourceCacheUses()).NotTo(BeZero())
							Expect(defaultBuild.Finish(db.BuildStatusFailed)).To(Succeed())
							Expect(buildCollector.Run(context.TODO())).To(Succeed())
							Expect(collector.Run(context.TODO())).To(Succeed())
							Expect(countResourceCacheUses()).To(BeZero())
						})
					})
				})
			})

			Context("when the build is for a job", func() {
				var jobBuild db.Build

				BeforeEach(func() {
					var err error
					jobBuild, err = defaultJob.CreateBuild("someone")
					Expect(err).ToNot(HaveOccurred())

					customResourceTypeCache, err = resourceCacheFactory.FindOrCreateResourceCache(
						db.ForBuild(jobBuild.ID()),
						"some-base-type",
						atc.Version{"some-type": "version"},
						atc.Source{
							"some-type": "source-param",
						},
						nil,
						nil,
					)
					Expect(err).NotTo(HaveOccurred())

					_, err = resourceCacheFactory.FindOrCreateResourceCache(
						db.ForBuild(jobBuild.ID()),
						"some-type",
						atc.Version{"some": "version"},
						atc.Source{
							"some": "source",
						},
						atc.Params{"some": "params"},
						customResourceTypeCache,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when it is the latest failed build", func() {
					It("preserves the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						Expect(jobBuild.Finish(db.BuildStatusFailed)).To(Succeed())
						Expect(buildCollector.Run(context.TODO())).To(Succeed())
						Expect(collector.Run(context.TODO())).To(Succeed())
						Expect(countResourceCacheUses()).NotTo(BeZero())
					})
				})

				Context("when a later build of the same job has succeeded", func() {
					var secondJobBuild db.Build

					BeforeEach(func() {
						var err error
						secondJobBuild, err = defaultJob.CreateBuild("someone")
						Expect(err).ToNot(HaveOccurred())

						_, err = resourceCacheFactory.FindOrCreateResourceCache(
							db.ForBuild(secondJobBuild.ID()),
							"some-type",
							atc.Version{"some": "version"},
							atc.Source{
								"some": "source",
							},
							atc.Params{"some": "params"},
							customResourceTypeCache,
						)
						Expect(err).NotTo(HaveOccurred())
					})

					It("cleans up the uses", func() {
						Expect(jobBuild.Finish(db.BuildStatusFailed)).To(Succeed())
						Expect(buildCollector.Run(context.TODO())).To(Succeed())
						Expect(collector.Run(context.TODO())).To(Succeed())

						Expect(countResourceCacheUses()).NotTo(BeZero())

						Expect(secondJobBuild.Finish(db.BuildStatusSucceeded)).To(Succeed())
						Expect(buildCollector.Run(context.TODO())).To(Succeed())
						Expect(collector.Run(context.TODO())).To(Succeed())

						Expect(countResourceCacheUses()).To(BeZero())
					})
				})
			})
		})
	})
})
