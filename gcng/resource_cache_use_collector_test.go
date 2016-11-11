package gcng_test

import (
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/gcng"
	sq "github.com/masterminds/squirrel"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

var _ = Describe("ResourceCacheUseCollector", func() {
	var (
		collector gcng.ResourceCacheUseCollector

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

		resourceType1     atc.ResourceType
		resourceType1Used *dbng.UsedResourceType
		usedResource      *dbng.Resource
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
			`{"some": "config"}`,
		)
		Expect(err).NotTo(HaveOccurred())

		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		baseResourceType := dbng.BaseResourceType{
			Name: "some-base-type",
		}
		_, err = baseResourceType.FindOrCreate(setupTx)
		Expect(err).NotTo(HaveOccurred())

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

		resourceCacheFactory = dbng.NewResourceCacheFactory(dbConn)

		logger := lagertest.NewTestLogger("resource-cache-use-collector")
		collector = gcng.NewResourceCacheUseCollector(logger, resourceCacheFactory)
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Run", func() {
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

		Describe("cache uses for builds", func() {
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
					collector.Run()
					Expect(countResourceCacheUses()).NotTo(BeZero())
				})
			})

			Context("once the build has completed successfully", func() {
				It("cleans up the uses", func() {
					Expect(countResourceCacheUses()).NotTo(BeZero())
					finishBuild("succeeded")
					collector.Run()
					Expect(countResourceCacheUses()).To(BeZero())
				})
			})

			Context("once the build has been aborted", func() {
				It("cleans up the uses", func() {
					Expect(countResourceCacheUses()).NotTo(BeZero())
					finishBuild("aborted")
					collector.Run()
					Expect(countResourceCacheUses()).To(BeZero())
				})
			})

			Context("once the build has failed", func() {
				It("cleans up the uses", func() {
					Expect(countResourceCacheUses()).NotTo(BeZero())
					finishBuild("failed")
					collector.Run()
					Expect(countResourceCacheUses()).To(BeZero())
				})
			})
		})

		Describe("cache uses for resource types", func() {
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
					collector.Run()
					Expect(countResourceCacheUses()).NotTo(BeZero())
				})
			})

			Context("once the resource type is made inactive", func() {
				It("cleans up the uses", func() {
					Expect(countResourceCacheUses()).NotTo(BeZero())
					setActiveResourceType(false)
					collector.Run()
					Expect(countResourceCacheUses()).To(BeZero())
				})
			})
		})

		Describe("cache uses for resource types", func() {
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
					collector.Run()
					Expect(countResourceCacheUses()).NotTo(BeZero())
				})
			})

			Context("once the resource is made inactive", func() {
				It("cleans up the uses", func() {
					Expect(countResourceCacheUses()).NotTo(BeZero())
					setActiveResource(false)
					collector.Run()
					Expect(countResourceCacheUses()).To(BeZero())
				})
			})
		})
	})
})
