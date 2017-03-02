package gcng_test

import (
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/gcng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceCacheUseCollector", func() {
	var collector gcng.Collector

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("resource-cache-use-collector")
		collector = gcng.NewResourceCacheUseCollector(logger, resourceCacheFactory)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Run", func() {
		Describe("cache uses", func() {
			var (
				resourceType1     atc.ResourceType
				resourceType1Used *dbng.UsedResourceType
			)

			countResourceCacheUses := func() int {
				tx, err := dbConn.Begin()
				Expect(err).NotTo(HaveOccurred())
				defer tx.Rollback()

				var result int
				err = psql.Select("count(*)").
					From("resource_cache_uses").
					RunWith(tx).
					QueryRow().
					Scan(&result)
				Expect(err).NotTo(HaveOccurred())

				return result
			}

			finishBuild := func(build dbng.Build, status string) {
				tx, err := dbConn.Begin()
				Expect(err).NotTo(HaveOccurred())
				defer tx.Rollback()

				var result time.Time
				err = psql.Update("builds").
					SetMap(map[string]interface{}{
						"status":    status,
						"end_time":  sq.Expr("NOW()"),
						"completed": true,
					}).Where(sq.Eq{
					"id": build.ID(),
				}).Suffix("RETURNING end_time").
					RunWith(tx).
					QueryRow().Scan(&result)
				Expect(err).NotTo(HaveOccurred())

				Expect(tx.Commit()).NotTo(HaveOccurred())
			}

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())
				resourceType1 = atc.ResourceType{
					Name: "some-type",
					Type: "some-base-type",
					Source: atc.Source{
						"some-type": "source",
					},
				}
				resourceType1Used, err = dbng.ResourceType{
					ResourceType: resourceType1,
					PipelineID:   defaultPipeline.ID(),
				}.Create(setupTx, atc.Version{"some-type": "version"})
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())
			})

			Describe("for one-off builds", func() {
				BeforeEach(func() {
					_, err = resourceCacheFactory.FindOrCreateResourceCache(
						logger,
						dbng.ForBuild{defaultBuild.ID()},
						"some-type",
						atc.Version{"some": "version"},
						atc.Source{
							"some": "source",
						},
						atc.Params{"some": "params"},
						defaultPipeline.ID(),
						atc.ResourceTypes{
							resourceType1,
						},
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
						finishBuild(defaultBuild, "succeeded")
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).To(BeZero())
					})
				})

				Context("once the build has been aborted", func() {
					It("cleans up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						finishBuild(defaultBuild, "aborted")
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).To(BeZero())
					})
				})

				Context("once the build has failed", func() {
					Context("when the build is a one-off", func() {
						It("cleans up the uses", func() {
							Expect(countResourceCacheUses()).NotTo(BeZero())
							finishBuild(defaultBuild, "failed")
							Expect(collector.Run()).To(Succeed())
							Expect(countResourceCacheUses()).To(BeZero())
						})
					})

					Context("when the build is for a job", func() {
						var jobId int

						BeforeEach(func() {
							tx, err := dbConn.Begin()
							Expect(err).NotTo(HaveOccurred())
							defer tx.Rollback()
							err = psql.Insert("jobs").
								Columns("name", "pipeline_id", "config").
								Values("lousy-job", defaultPipeline.ID(), `{"some":"config"}`).
								Suffix("RETURNING id").
								RunWith(tx).QueryRow().Scan(&jobId)
							Expect(err).NotTo(HaveOccurred())
							Expect(tx.Commit()).To(Succeed())
						})

						JustBeforeEach(func() {
							tx, err := dbConn.Begin()
							Expect(err).NotTo(HaveOccurred())
							defer tx.Rollback()
							_, err = psql.Update("builds").
								SetMap(map[string]interface{}{
									"status":    "failed",
									"end_time":  sq.Expr("NOW()"),
									"completed": true,
									"job_id":    jobId,
								}).
								RunWith(tx).Exec()
							Expect(err).NotTo(HaveOccurred())
							Expect(tx.Commit()).To(Succeed())
						})

						Context("when it is the latest failed build", func() {
							It("preserves the uses", func() {
								Expect(countResourceCacheUses()).NotTo(BeZero())
								finishBuild(defaultBuild, "failed")
								Expect(collector.Run()).To(Succeed())
								Expect(countResourceCacheUses()).NotTo(BeZero())
							})
						})

						Context("when a later build of the same job has failed also", func() {
							BeforeEach(func() {
								_, err = defaultTeam.CreateOneOffBuild()
								Expect(err).NotTo(HaveOccurred())
							})

							It("cleans up the uses", func() {
								Expect(countResourceCacheUses()).NotTo(BeZero())
								Expect(collector.Run()).To(Succeed())
								Expect(countResourceCacheUses()).To(BeZero())
							})
						})
					})
				})

				Context("if build is using image resource", func() {
					BeforeEach(func() {
						err := defaultBuild.SaveImageResourceVersion(atc.PlanID("123"), atc.Version{"ref": "abc"}, "some-resource-hash")
						Expect(err).NotTo(HaveOccurred())
					})

					It("deletes the use for old build image resource", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						finishBuild(defaultBuild, "succeeded")
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).To(BeZero())
					})
				})
			})

			Context("for pipeline builds", func() {
				Context("if job build is using image resource", func() {
					var firstBuild dbng.Build

					BeforeEach(func() {
						err = defaultPipeline.SaveJob(atc.JobConfig{
							Name: "some-job",
						})
						Expect(err).NotTo(HaveOccurred())

						firstBuild, err = defaultPipeline.CreateJobBuild("some-job")
						Expect(err).NotTo(HaveOccurred())

						imageVersion := atc.Version{"ref": "abc"}
						err = firstBuild.SaveImageResourceVersion(atc.PlanID("123"), imageVersion, "some-resource-hash")
						Expect(err).NotTo(HaveOccurred())

						_, err = resourceCacheFactory.FindOrCreateResourceCache(
							logger,
							dbng.ForBuild{firstBuild.ID()},
							"some-base-type",
							imageVersion,
							atc.Source{
								"some": "source",
							},
							nil,
							defaultPipeline.ID(),
							atc.ResourceTypes{},
						)
						Expect(err).NotTo(HaveOccurred())

						finishBuild(firstBuild, "succeeded")
					})

					It("keeps the use for latest build image resource", func() {
						Expect(countResourceCacheUses()).To(Equal(1))
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).To(Equal(1))
					})

					It("deletes the use for old build image resource", func() {
						secondBuild, err := defaultPipeline.CreateJobBuild("some-job")
						Expect(err).NotTo(HaveOccurred())

						imageVersion2 := atc.Version{"ref": "abc2"}
						err = secondBuild.SaveImageResourceVersion(atc.PlanID("123"), imageVersion2, "some-resource-hash")
						Expect(err).NotTo(HaveOccurred())

						finishBuild(secondBuild, "succeeded")

						_, err = resourceCacheFactory.FindOrCreateResourceCache(
							logger,
							dbng.ForBuild{secondBuild.ID()},
							"some-base-type",
							imageVersion2,
							atc.Source{
								"some": "source",
							},
							nil,
							defaultPipeline.ID(),
							atc.ResourceTypes{},
						)
						Expect(err).NotTo(HaveOccurred())

						Expect(countResourceCacheUses()).To(Equal(2))

						Expect(collector.Run()).To(Succeed())

						Expect(countResourceCacheUses()).To(Equal(1))

						tx, err := dbConn.Begin()
						Expect(err).NotTo(HaveOccurred())
						defer tx.Rollback()

						var buildID int
						err = psql.Select("build_id").
							From("resource_cache_uses").
							RunWith(tx).
							QueryRow().
							Scan(&buildID)
						Expect(err).NotTo(HaveOccurred())

						Expect(buildID).To(Equal(secondBuild.ID()))
					})
				})
			})

			Describe("for resource types", func() {
				setActiveResourceType := func(active bool) {
					tx, err := dbConn.Begin()
					Expect(err).NotTo(HaveOccurred())
					defer tx.Rollback()

					var id int
					err = psql.Update("resource_types").
						Set("active", active).
						Where(sq.Eq{
							"id": resourceType1Used.ID,
						}).Suffix("RETURNING id").
						RunWith(tx).
						QueryRow().Scan(&id)
					Expect(err).NotTo(HaveOccurred())

					err = tx.Commit()
					Expect(err).NotTo(HaveOccurred())
				}

				BeforeEach(func() {
					_, err = resourceCacheFactory.FindOrCreateResourceCache(
						logger,
						dbng.ForResourceType{resourceType1Used.ID},
						"some-type",
						atc.Version{"some-type": "version"},
						atc.Source{
							"cache": "source",
						},
						atc.Params{"some": "params"},
						defaultPipeline.ID(),
						atc.ResourceTypes{
							resourceType1,
						},
					)
					Expect(err).NotTo(HaveOccurred())
					setActiveResourceType(true)
				})

				Context("while the resource type is still active", func() {
					It("does not clean up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).NotTo(BeZero())
					})
				})

				Context("once the resource type is made inactive", func() {
					It("cleans up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						setActiveResourceType(false)
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).To(BeZero())
					})
				})
			})

			Describe("for resources", func() {
				setActiveResource := func(active bool) {
					tx, err := dbConn.Begin()
					Expect(err).NotTo(HaveOccurred())
					defer tx.Rollback()

					var id int
					err = psql.Update("resources").
						Set("active", active).
						Where(sq.Eq{
							"id": usedResource.ID,
						}).Suffix("RETURNING id").
						RunWith(tx).
						QueryRow().Scan(&id)
					Expect(err).NotTo(HaveOccurred())

					err = tx.Commit()
					Expect(err).NotTo(HaveOccurred())
				}

				BeforeEach(func() {
					_, err = resourceCacheFactory.FindOrCreateResourceCache(
						logger,
						dbng.ForResource{usedResource.ID},
						"some-type",
						atc.Version{"some-type": "version"},
						atc.Source{
							"cache": "source",
						},
						atc.Params{"some": "params"},
						defaultPipeline.ID(),
						atc.ResourceTypes{
							resourceType1,
						},
					)
					Expect(err).NotTo(HaveOccurred())
					setActiveResource(true)
				})

				Context("while the resource is still active", func() {
					It("does not clean up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).NotTo(BeZero())
					})
				})

				Context("once the resource is made inactive", func() {
					It("cleans up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						setActiveResource(false)
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).To(BeZero())
					})
				})
			})
		})
	})
})
