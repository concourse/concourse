package gc_test

import (
	"context"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/gc"

	. "github.com/onsi/ginkgo"
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
				versionedResourceType atc.VersionedResourceType
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

			BeforeEach(func() {
				versionedResourceType = atc.VersionedResourceType{
					ResourceType: atc.ResourceType{
						Name: "some-type",
						Type: "some-base-type",
						Source: atc.Source{
							"some-type": "source-param",
						},
					},
					Version: atc.Version{"some-type": "version"},
				}
			})

			Describe("for one-off builds", func() {
				BeforeEach(func() {
					_, err = resourceCacheFactory.FindOrCreateResourceCache(
						db.ForBuild(defaultBuild.ID()),
						"some-type",
						atc.Version{"some": "version"},
						atc.Source{
							"some": "source",
						},
						atc.Params{"some": "params"},
						atc.VersionedResourceTypes{
							versionedResourceType,
						},
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
					jobBuild, err = defaultJob.CreateBuild()
					Expect(err).ToNot(HaveOccurred())

					_, err = resourceCacheFactory.FindOrCreateResourceCache(
						db.ForBuild(jobBuild.ID()),
						"some-type",
						atc.Version{"some": "version"},
						atc.Source{
							"some": "source",
						},
						atc.Params{"some": "params"},
						atc.VersionedResourceTypes{
							versionedResourceType,
						},
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
						secondJobBuild, err = defaultJob.CreateBuild()
						Expect(err).ToNot(HaveOccurred())

						_, err = resourceCacheFactory.FindOrCreateResourceCache(
							db.ForBuild(secondJobBuild.ID()),
							"some-type",
							atc.Version{"some": "version"},
							atc.Source{
								"some": "source",
							},
							atc.Params{"some": "params"},
							atc.VersionedResourceTypes{
								versionedResourceType,
							},
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

			Describe("for containers", func() {
				var container db.CreatingContainer

				BeforeEach(func() {
					worker, err := defaultTeam.SaveWorker(atc.Worker{
						Name: "some-worker",
					}, 0)
					Expect(err).ToNot(HaveOccurred())

					container, err = worker.CreateContainer(
						db.NewBuildStepContainerOwner(defaultBuild.ID(), "some-plan", defaultTeam.ID()),
						db.ContainerMetadata{},
					)
					Expect(err).ToNot(HaveOccurred())

					_, err = resourceCacheFactory.FindOrCreateResourceCache(
						db.ForContainer(container.ID()),
						"some-type",
						atc.Version{"some-type": "version"},
						atc.Source{
							"cache": "source",
						},
						atc.Params{"some": "params"},
						atc.VersionedResourceTypes{
							versionedResourceType,
						},
					)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("while the container is still in use", func() {
					It("does not clean up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						Expect(collector.Run(context.TODO())).To(Succeed())
						Expect(countResourceCacheUses()).NotTo(BeZero())
					})
				})

				Context("when the container is removed", func() {
					It("cleans up the uses (except it was actually a cascade delete, not the GC, lol)", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						created, err := container.Created()
						Expect(err).ToNot(HaveOccurred())
						destroying, err := created.Destroying()
						Expect(err).ToNot(HaveOccurred())
						Expect(destroying.Destroy()).To(BeTrue())
						Expect(collector.Run(context.TODO())).To(Succeed())
						Expect(countResourceCacheUses()).To(BeZero())
					})
				})
			})
		})
	})
})
