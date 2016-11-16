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

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

var _ = Describe("ResourceCacheCollector", func() {
	var (
		collector gcng.ResourceCacheCollector

		dbConn               dbng.Conn
		err                  error
		resourceCacheFactory dbng.ResourceCacheFactory

		teamFactory     dbng.TeamFactory
		buildFactory    *dbng.BuildFactory
		pipelineFactory *dbng.PipelineFactory
		resourceFactory *dbng.ResourceFactory

		defaultTeam     *dbng.Team
		defaultPipeline *dbng.Pipeline
		defaultBuild    *dbng.Build

		usedResource *dbng.Resource
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = dbng.Wrap(postgresRunner.Open())

		teamFactory = dbng.NewTeamFactory(dbConn)
		buildFactory = dbng.NewBuildFactory(dbConn)
		pipelineFactory = dbng.NewPipelineFactory(dbConn)
		resourceFactory = dbng.NewResourceFactory(dbConn)

		defaultTeam, err = teamFactory.CreateTeam("default-team")
		Expect(err).NotTo(HaveOccurred())

		defaultBuild, err = buildFactory.CreateOneOffBuild(defaultTeam)
		Expect(err).NotTo(HaveOccurred())

		defaultPipeline, err = pipelineFactory.CreatePipeline(defaultTeam, "default-pipeline", "some-config")
		Expect(err).NotTo(HaveOccurred())

		usedResource, err = resourceFactory.CreateResource(
			defaultPipeline,
			"some-resource",
			`{"some":"source"}`,
		)
		Expect(err).NotTo(HaveOccurred())

		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		baseResourceType := dbng.BaseResourceType{
			Name: "some-base-type",
		}
		_, err = baseResourceType.FindOrCreate(setupTx)
		Expect(err).NotTo(HaveOccurred())

		Expect(setupTx.Commit()).To(Succeed())

		resourceCacheFactory = dbng.NewResourceCacheFactory(dbConn)

		logger := lagertest.NewTestLogger("resource-cache-use-collector")
		collector = gcng.NewResourceCacheCollector(logger, resourceCacheFactory)
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
					Pipeline:     defaultPipeline,
					Version:      atc.Version{"some-type": "version"},
				}.Create(setupTx)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())
			})

			Describe("for builds", func() {
				BeforeEach(func() {
					_, err = resourceCacheFactory.FindOrCreateResourceCacheForBuild(
						defaultBuild,
						"some-type",
						atc.Version{"some": "version"},
						atc.Source{
							"some": "source",
						},
						atc.Params{"some": "params"},
						defaultPipeline,
						atc.ResourceTypes{
							resourceType1,
						},
					)
					Expect(err).NotTo(HaveOccurred())
				})

				finishBuild := func(status string) {
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
						"id": defaultBuild.ID,
					}).Suffix("RETURNING end_time").
						RunWith(tx).
						QueryRow().Scan(&result)
					Expect(err).NotTo(HaveOccurred())

					err = tx.Commit()
					Expect(err).NotTo(HaveOccurred())
				}

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
						finishBuild("succeeded")
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).To(BeZero())
					})
				})

				Context("once the build has been aborted", func() {
					It("cleans up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						finishBuild("aborted")
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).To(BeZero())
					})
				})

				FContext("once the build has failed", func() {
					It("cleans up the uses", func() {
						Expect(countResourceCacheUses()).NotTo(BeZero())
						finishBuild("failed")
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCacheUses()).To(BeZero())
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
					_, err = resourceCacheFactory.FindOrCreateResourceCacheForResourceType(
						"some-type",
						atc.Version{"some-type": "version"},
						atc.Source{
							"cache": "source",
						},
						atc.Params{"some": "params"},
						defaultPipeline,
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
					_, err = resourceCacheFactory.FindOrCreateResourceCacheForResource(
						usedResource,
						"some-type",
						atc.Version{"some-type": "version"},
						atc.Source{
							"cache": "source",
						},
						atc.Params{"some": "params"},
						defaultPipeline,
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
				_, err = resourceCacheFactory.FindOrCreateResourceCacheForBuild(
					defaultBuild,
					"some-base-type",
					atc.Version{"some": "version"},
					atc.Source{
						"some": "source",
					},
					nil,
					defaultPipeline,
					atc.ResourceTypes{},
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
				JustBeforeEach(func() {
					tx, err := dbConn.Begin()
					Expect(err).NotTo(HaveOccurred())
					defer tx.Rollback()

					_, err = psql.Delete("resource_cache_uses").
						RunWith(tx).Exec()
					Expect(err).NotTo(HaveOccurred())

					err = tx.Commit()
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
							Values("lousy-job", defaultPipeline.ID, `{"some":"config"}`).
							Suffix("RETURNING id").
							RunWith(tx).QueryRow().Scan(&jobId)
						Expect(err).NotTo(HaveOccurred())
						var versionId int
						err = psql.Insert("versioned_resources").
							Columns("version", "metadata", "type", "resource_id").
							Values(`{"some":"version"}`, `[]`, "whatever", usedResource.ID).
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

					It("leaves it alone", func() {
						Expect(collector.Run()).To(Succeed())
						Expect(countResourceCaches()).NotTo(BeZero())
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
								Values(`{"some":"version"}`, defaultBuild.ID, "whatever", "whatever").
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
									Values("lousy-job", defaultPipeline.ID, `{"some":"config"}`).
									Suffix("RETURNING id").
									RunWith(tx).QueryRow().Scan(&jobId)
								Expect(err).NotTo(HaveOccurred())

								_, err = psql.Update("builds").
									Set("job_id", jobId).
									Where(sq.Eq{"id": defaultBuild.ID}).
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
									newBuild, err := buildFactory.CreateOneOffBuild(defaultTeam)
									Expect(err).NotTo(HaveOccurred())
									_, err = resourceCacheFactory.FindOrCreateResourceCacheForBuild(
										newBuild,
										"some-base-type",
										atc.Version{"new": "version"},
										atc.Source{
											"some": "source",
										},
										nil,
										defaultPipeline,
										atc.ResourceTypes{},
									)
									Expect(err).NotTo(HaveOccurred())

									tx, err := dbConn.Begin()
									Expect(err).NotTo(HaveOccurred())
									defer tx.Rollback()
									Expect(err).NotTo(HaveOccurred())
									_, err = psql.Insert("image_resource_versions").
										Columns("version", "build_id", "plan_id", "resource_hash").
										Values(`{"new":"version"}`, newBuild.ID, "whatever", "whatever").
										RunWith(tx).Exec()
									Expect(err).NotTo(HaveOccurred())
									_, err = psql.Update("builds").
										Set("job_id", jobId).
										Where(sq.Eq{"id": newBuild.ID}).
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
							It("is not preserved", func() {
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
