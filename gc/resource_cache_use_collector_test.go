package gc_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/gc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceCacheUseCollector", func() {
	var collector gc.Collector
	var buildCollector gc.Collector

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("resource-cache-use-collector")
		collector = gc.NewResourceCacheUseCollector(logger, resourceCacheLifecycle)
		buildCollector = gc.NewBuildCollector(logger, buildFactory)
	})

	Describe("Run", func() {
		Describe("cache uses", func() {
			var (
				pipelineWithTypes     db.Pipeline
				versionedResourceType atc.VersionedResourceType
				dbResourceType        db.ResourceType
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
							"some-type": "((source-param))",
						},
					},
					Version: atc.Version{"some-type": "version"},
				}

				var created bool
				var err error
				pipelineWithTypes, created, err = defaultTeam.SavePipeline(
					"pipeline-with-types",
					atc.Config{
						ResourceTypes: atc.ResourceTypes{versionedResourceType.ResourceType},
					},
					0,
					db.PipelineNoChange,
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())

				var found bool
				dbResourceType, found, err = pipelineWithTypes.ResourceType("some-type")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(dbResourceType.SaveVersion(versionedResourceType.Version)).To(Succeed())
				Expect(dbResourceType.Reload()).To(BeTrue())
			})

			Describe("for one-off builds", func() {
				BeforeEach(func() {
					_, err = resourceCacheFactory.FindOrCreateResourceCache(
						logger,
						db.ForBuild(defaultBuild.ID()),
						"some-type",
						atc.Version{"some": "version"},
						atc.Source{
							"some": "source",
						},
						atc.Params{"some": "params"},
						creds.NewVersionedResourceTypes(template.StaticVariables{"source-param": "some-secret-sauce"},
							atc.VersionedResourceTypes{
								versionedResourceType,
							},
						),
					)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("before the build has completed", func() {
					It("does not clean up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).NotTo(BeZero())
					})
				})

				Context("once the build has completed successfully", func() {
					It("cleans up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						Expect(defaultBuild.Finish(db.BuildStatusSucceeded)).To(Succeed())
						Expect(buildCollector.Run()).To(Succeed())
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).To(BeZero())
					})
				})

				Context("once the build has been aborted", func() {
					It("cleans up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						Expect(defaultBuild.Finish(db.BuildStatusAborted)).To(Succeed())
						Expect(buildCollector.Run()).To(Succeed())
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).To(BeZero())
					})
				})

				Context("once the build has failed", func() {
					Context("when the build is a one-off", func() {
						It("cleans up the uses", func() {
							Expect(countResourceCacheUses()).NotTo(BeZero())
							Expect(defaultBuild.Finish(db.BuildStatusFailed)).To(Succeed())
							Expect(buildCollector.Run()).To(Succeed())
							Expect(collector.Run()).To(Succeed())
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
						logger,
						db.ForBuild(jobBuild.ID()),
						"some-type",
						atc.Version{"some": "version"},
						atc.Source{
							"some": "source",
						},
						atc.Params{"some": "params"},
						creds.NewVersionedResourceTypes(template.StaticVariables{"source-param": "some-secret-sauce"},
							atc.VersionedResourceTypes{
								versionedResourceType,
							},
						),
					)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when it is the latest failed build", func() {
					It("preserves the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						Expect(jobBuild.Finish(db.BuildStatusFailed)).To(Succeed())
						Expect(buildCollector.Run()).To(Succeed())
						Expect(collector.Run()).To(Succeed())
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
							logger,
							db.ForBuild(secondJobBuild.ID()),
							"some-type",
							atc.Version{"some": "version"},
							atc.Source{
								"some": "source",
							},
							atc.Params{"some": "params"},
							creds.NewVersionedResourceTypes(template.StaticVariables{"source-param": "some-secret-sauce"},
								atc.VersionedResourceTypes{
									versionedResourceType,
								},
							),
						)
						Expect(err).NotTo(HaveOccurred())
					})

					It("cleans up the uses", func() {
						Expect(jobBuild.Finish(db.BuildStatusFailed)).To(Succeed())
						Expect(buildCollector.Run()).To(Succeed())
						Expect(collector.Run()).To(Succeed())

						Expect(countResourceCacheUses()).NotTo(BeZero())

						Expect(secondJobBuild.Finish(db.BuildStatusSucceeded)).To(Succeed())
						Expect(buildCollector.Run()).To(Succeed())
						Expect(collector.Run()).To(Succeed())

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

					container, err = defaultTeam.CreateContainer(
						worker.Name(),
						db.NewBuildStepContainerOwner(defaultBuild.ID(), "some-plan"),
						db.ContainerMetadata{},
					)
					Expect(err).ToNot(HaveOccurred())

					_, err = resourceCacheFactory.FindOrCreateResourceCache(
						logger,
						db.ForContainer(container.ID()),
						"some-type",
						atc.Version{"some-type": "version"},
						atc.Source{
							"cache": "source",
						},
						atc.Params{"some": "params"},
						creds.NewVersionedResourceTypes(template.StaticVariables{"source-param": "some-secret-sauce"},
							atc.VersionedResourceTypes{
								versionedResourceType,
							},
						),
					)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("while the container is still in use", func() {
					It("does not clean up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						Expect(collector.Run()).To(Succeed())
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
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).To(BeZero())
					})
				})
			})
		})
	})
})
