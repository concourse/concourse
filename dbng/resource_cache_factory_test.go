package dbng_test

import (
	"database/sql"

	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceCacheFactory", func() {
	var (
		dbConn               dbng.Conn
		resourceCacheFactory dbng.ResourceCacheFactory
		build                *dbng.Build
		pipeline             *dbng.Pipeline

		usedBaseResourceType *dbng.UsedBaseResourceType

		resourceType1 atc.ResourceType
		resourceType2 atc.ResourceType
		resourceType3 atc.ResourceType
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = dbng.Wrap(postgresRunner.Open())
		resourceCacheFactory = dbng.NewResourceCacheFactory(dbConn)
		teamFactory := dbng.NewTeamFactory(dbConn)
		team, err := teamFactory.CreateTeam("some-team")
		Expect(err).ToNot(HaveOccurred())

		buildFactory := dbng.NewBuildFactory(dbConn)
		build, err = buildFactory.CreateOneOffBuild(team)
		Expect(err).ToNot(HaveOccurred())

		pipelineFactory := dbng.NewPipelineFactory(dbConn)
		pipeline, err = pipelineFactory.CreatePipeline(team, "some-pipeline", "{}")
		Expect(err).ToNot(HaveOccurred())

		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		baseResourceType := dbng.BaseResourceType{
			Name: "some-base-type",
		}
		usedBaseResourceType, err = baseResourceType.FindOrCreate(setupTx)
		Expect(err).NotTo(HaveOccurred())

		resourceType1 = atc.ResourceType{
			Name: "some-type",
			Type: "some-type-type",
			Source: atc.Source{
				"some-type": "source",
			},
		}
		_, err = dbng.ResourceType{
			ResourceType: resourceType1,
			Pipeline:     pipeline,
			Version:      atc.Version{"some-type": "version"},
		}.Create(setupTx)
		Expect(err).NotTo(HaveOccurred())

		resourceType2 = atc.ResourceType{
			Name: "some-type-type",
			Type: "some-base-type",
			Source: atc.Source{
				"some-type-type": "source",
			},
		}
		_, err = dbng.ResourceType{
			ResourceType: resourceType2,
			Pipeline:     pipeline,
			Version:      atc.Version{"some-type-type": "version"},
		}.Create(setupTx)
		Expect(err).NotTo(HaveOccurred())

		resourceType3 = atc.ResourceType{
			Name: "some-unused-type",
			Type: "some-base-type",
		}
		_, err = dbng.ResourceType{
			ResourceType: resourceType3,
			Pipeline:     pipeline,
			Version:      atc.Version{"some-unused-type": "version"},
		}.Create(setupTx)
		Expect(err).NotTo(HaveOccurred())

		Expect(setupTx.Commit()).To(Succeed())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("FindOrCreateResourceCacheForBuild", func() {
		It("creates resource cache in database", func() {
			_, err := resourceCacheFactory.FindOrCreateResourceCacheForBuild(
				build,
				"some-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				pipeline,
				atc.ResourceTypes{
					resourceType1,
					resourceType2,
					resourceType3,
				},
			)
			Expect(err).ToNot(HaveOccurred())

			tx, err := dbConn.Begin()
			Expect(err).NotTo(HaveOccurred())
			defer tx.Rollback()

			rows, err := psql.Select("a.version, a.params_hash, o.source_hash, b.name").
				From("resource_caches a").
				LeftJoin("resource_configs o ON a.resource_config_id = o.id").
				LeftJoin("base_resource_types b ON o.base_resource_type_id = b.id").
				RunWith(tx).
				Query()
			Expect(err).NotTo(HaveOccurred())
			resourceCaches := []resourceCache{}
			for rows.Next() {
				var version string
				var paramsHash string
				var sourceHash sql.NullString
				var baseResourceTypeName sql.NullString

				err := rows.Scan(&version, &paramsHash, &sourceHash, &baseResourceTypeName)
				Expect(err).NotTo(HaveOccurred())

				var sourceHashString string
				if sourceHash.Valid {
					sourceHashString = sourceHash.String
				}

				var baseResourceTypeNameString string
				if baseResourceTypeName.Valid {
					baseResourceTypeNameString = baseResourceTypeName.String
				}

				resourceCaches = append(resourceCaches, resourceCache{
					Version:          version,
					ParamsHash:       paramsHash,
					SourceHash:       sourceHashString,
					BaseResourceName: baseResourceTypeNameString,
				})
			}

			Expect(resourceCaches).To(ConsistOf(
				resourceCache{
					Version:          `{"some-type-type":"version"}`,
					ParamsHash:       "null",
					BaseResourceName: "some-base-type",
					SourceHash:       `{"some-type-type":"source"}`,
				},
				resourceCache{
					Version:    `{"some-type":"version"}`,
					ParamsHash: "null",
					SourceHash: `{"some-type":"source"}`,
				},
				resourceCache{
					Version:    `{"some":"version"}`,
					ParamsHash: `{"some":"params"}`,
					SourceHash: `{"some":"source"}`,
				},
			))
		})

		It("returns an error if base resource type does not exist", func() {
			_, err := resourceCacheFactory.FindOrCreateResourceCacheForBuild(
				build,
				"some-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				pipeline,
				atc.ResourceTypes{
					resourceType1,
					{
						Name: "some-type-type",
						Type: "non-existent-base-type",
						Source: atc.Source{
							"some-type-type": "source",
						},
					},
				},
			)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(dbng.ErrBaseResourceTypeNotFound))
		})
	})
})

type resourceCache struct {
	Version          string
	ParamsHash       string
	SourceHash       string
	BaseResourceName string
}
