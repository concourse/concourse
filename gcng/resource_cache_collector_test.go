package gcng_test

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/gcng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceCacheCollector", func() {
	var collector gcng.Collector
	var buildCollector gcng.Collector

	BeforeEach(func() {
		collector = gcng.NewResourceCacheCollector(logger, resourceCacheFactory)
		buildCollector = gcng.NewBuildCollector(logger, buildFactory)
	})

	Describe("Run", func() {
		Describe("resource caches", func() {
			countResourceCaches := func() int {
				tx, err := dbConn.Begin()
				Expect(err).NotTo(HaveOccurred())
				defer tx.Rollback()

				var result int
				err = psql.Select("count(*)").
					From("resource_caches").
					RunWith(tx).
					QueryRow().
					Scan(&result)
				Expect(err).NotTo(HaveOccurred())

				return result
			}

			BeforeEach(func() {
				_, err = resourceCacheFactory.FindOrCreateResourceCache(
					logger,
					dbng.ForBuild(defaultBuild.ID()),
					"some-base-type",
					atc.Version{"some": "version"},
					atc.Source{
						"some": "source",
					},
					nil,
					atc.VersionedResourceTypes{},
				)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				Expect(countResourceCaches()).NotTo(BeZero())
			})

			Context("when the resource cache is still in use", func() {
				It("does not delete the cache", func() {
					Expect(collector.Run()).To(Succeed())
					Expect(countResourceCaches()).NotTo(BeZero())
				})
			})

			Context("when the cache is no longer in use", func() {
				var resourceCacheUseCollector gcng.Collector

				JustBeforeEach(func() {
					err := defaultBuild.Finish(dbng.BuildStatusSucceeded)
					Expect(err).NotTo(HaveOccurred())
					Expect(buildCollector.Run()).To(Succeed())

					resourceCacheUseCollector = gcng.NewResourceCacheUseCollector(logger, resourceCacheFactory)
					err = resourceCacheUseCollector.Run()
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when the cache is a next_build_input", func() {
					BeforeEach(func() {
						tx, err := dbConn.Begin()
						Expect(err).NotTo(HaveOccurred())
						defer tx.Rollback()
						var jobId int
						err = psql.Insert("jobs").
							Columns("name", "pipeline_id", "config").
							Values("lousy-job", defaultPipeline.ID(), `{"some":"config"}`).
							Suffix("RETURNING id").
							RunWith(tx).QueryRow().Scan(&jobId)
						Expect(err).NotTo(HaveOccurred())
						var versionId int
						err = psql.Insert("versioned_resources").
							Columns("version", "metadata", "type", "resource_id").
							Values(`{"some":"version"}`, `[]`, "whatever", usedResource.ID()).
							Suffix("RETURNING id").
							RunWith(tx).QueryRow().Scan(&versionId)
						Expect(err).NotTo(HaveOccurred())
						_, err = psql.Insert("next_build_inputs").
							Columns("job_id", "input_name", "version_id", "first_occurrence").
							Values(jobId, "whatever", versionId, false).
							RunWith(tx).Exec()
						Expect(err).NotTo(HaveOccurred())
						Expect(tx.Commit()).NotTo(HaveOccurred())
					})

					Context("when pipeline is paused", func() {
						BeforeEach(func() {
							err := defaultPipeline.Pause()
							Expect(err).NotTo(HaveOccurred())
						})

						It("removes the cache", func() {
							Expect(collector.Run()).To(Succeed())
							Expect(countResourceCaches()).To(BeZero())
						})
					})

					Context("when pipeline is not paused", func() {
						It("leaves it alone", func() {
							Expect(collector.Run()).To(Succeed())
							Expect(countResourceCaches()).NotTo(BeZero())
						})
					})
				})

				Context("when the cache is not a next_build_input", func() {
					Context("when the cache is an image_resource_version", func() {
						BeforeEach(func() {
							tx, err := dbConn.Begin()
							Expect(err).NotTo(HaveOccurred())
							defer tx.Rollback()
							_, err = psql.Insert("image_resource_versions").
								Columns("version", "build_id", "plan_id", "resource_hash").
								Values(`{"some":"version"}`, defaultBuild.ID(), "whatever", "whatever").
								RunWith(tx).Exec()
							Expect(err).NotTo(HaveOccurred())
							Expect(tx.Commit()).NotTo(HaveOccurred())
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

								_, err = psql.Update("builds").
									Set("job_id", jobId).
									Where(sq.Eq{"id": defaultBuild.ID()}).
									RunWith(tx).Exec()
								Expect(err).NotTo(HaveOccurred())
								Expect(tx.Commit()).To(Succeed())
							})

							Context("when the cache is the latest image_resource_version", func() {
								It("leaves it alone", func() {
									Expect(collector.Run()).To(Succeed())
									Expect(countResourceCaches()).NotTo(BeZero())
								})
							})

							Context("when the cache is for an older image_resource_version", func() {
								BeforeEach(func() {
									newBuild, err := defaultTeam.CreateOneOffBuild()
									Expect(err).NotTo(HaveOccurred())
									_, err = resourceCacheFactory.FindOrCreateResourceCache(
										logger,
										dbng.ForBuild(newBuild.ID()),
										"some-base-type",
										atc.Version{"new": "version"},
										atc.Source{
											"some": "source",
										},
										nil,
										atc.VersionedResourceTypes{},
									)
									Expect(err).NotTo(HaveOccurred())

									err = newBuild.Finish("succeeded")
									Expect(err).NotTo(HaveOccurred())

									Expect(buildCollector.Run()).To(Succeed())

									tx, err := dbConn.Begin()
									Expect(err).NotTo(HaveOccurred())
									defer tx.Rollback()

									_, err = psql.Insert("image_resource_versions").
										Columns("version", "build_id", "plan_id", "resource_hash").
										Values(`{"new":"version"}`, newBuild.ID(), "whatever", "whatever").
										RunWith(tx).Exec()
									Expect(err).NotTo(HaveOccurred())
									_, err = psql.Update("builds").
										Set("job_id", jobId).
										Where(sq.Eq{"id": newBuild.ID()}).
										RunWith(tx).Exec()
									Expect(err).NotTo(HaveOccurred())
									Expect(tx.Commit()).To(Succeed())
								})

								It("preserves only the newest one", func() {
									Expect(collector.Run()).To(Succeed())
									Expect(countResourceCaches()).To(Equal(1))
									tx, err := dbConn.Begin()
									Expect(err).NotTo(HaveOccurred())
									defer tx.Rollback()

									var result int
									err = psql.Select("id").
										From("resource_caches").
										RunWith(tx).
										QueryRow().
										Scan(&result)
									Expect(err).NotTo(HaveOccurred())
									Expect(result).To(Equal(2))
								})
							})
						})

						Context("when the cache is for a one-off build", func() {
							It("is deleted after build is finished", func() {
								Expect(collector.Run()).To(Succeed())
								Expect(countResourceCaches()).To(BeZero())
							})
						})
					})

					Context("when the cache is not an image_resource_version", func() {
						It("cleans up the cache record", func() {
							Expect(collector.Run()).To(Succeed())
							Expect(countResourceCaches()).To(BeZero())
						})
					})
				})
			})
		})
	})
})
