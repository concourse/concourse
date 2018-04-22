package db_test

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sync"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceCacheFactory", func() {
	var (
		usedImageBaseResourceType *db.UsedBaseResourceType

		resourceCacheLifecycle db.ResourceCacheLifecycle

		resourceType1                  atc.VersionedResourceType
		resourceType2                  atc.VersionedResourceType
		resourceType3                  atc.VersionedResourceType
		resourceTypeUsingBogusBaseType atc.VersionedResourceType
		resourceTypeOverridingBaseType atc.VersionedResourceType

		logger *lagertest.TestLogger
		build  db.Build
	)

	BeforeEach(func() {
		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		baseResourceType := db.BaseResourceType{
			Name: "some-base-type",
		}

		_, err = baseResourceType.FindOrCreate(setupTx)
		Expect(err).NotTo(HaveOccurred())

		imageBaseResourceType := db.BaseResourceType{
			Name: "some-image-type",
		}

		resourceCacheLifecycle = db.NewResourceCacheLifecycle(dbConn)

		usedImageBaseResourceType, err = imageBaseResourceType.FindOrCreate(setupTx)
		Expect(err).NotTo(HaveOccurred())

		Expect(setupTx.Commit()).To(Succeed())

		resourceType1 = atc.VersionedResourceType{
			ResourceType: atc.ResourceType{
				Name: "some-type",
				Type: "some-type-type",
				Source: atc.Source{
					"some-type": "source",
				},
			},
			Version: atc.Version{"some-type": "version"},
		}

		resourceType2 = atc.VersionedResourceType{
			ResourceType: atc.ResourceType{
				Name: "some-type-type",
				Type: "some-base-type",
				Source: atc.Source{
					"some-type-type": "((source-param))",
				},
			},
			Version: atc.Version{"some-type-type": "version"},
		}

		resourceType3 = atc.VersionedResourceType{
			ResourceType: atc.ResourceType{
				Name: "some-unused-type",
				Type: "some-base-type",
				Source: atc.Source{
					"some-unused-type": "source",
				},
			},
			Version: atc.Version{"some-unused-type": "version"},
		}

		resourceTypeUsingBogusBaseType = atc.VersionedResourceType{
			ResourceType: atc.ResourceType{
				Name: "some-type-using-bogus-base-type",
				Type: "some-bogus-base-type",
				Source: atc.Source{
					"some-type-using-bogus-base-type": "source",
				},
			},
			Version: atc.Version{"some-type-using-bogus-base-type": "version"},
		}

		resourceTypeOverridingBaseType = atc.VersionedResourceType{
			ResourceType: atc.ResourceType{
				Name: "some-image-type",
				Type: "some-image-type",
				Source: atc.Source{
					"some-image-type": "source",
				},
			},
			Version: atc.Version{"some-image-type": "version"},
		}

		pipelineWithTypes, _, err := defaultTeam.SavePipeline(
			"pipeline-with-types",
			atc.Config{
				ResourceTypes: atc.ResourceTypes{
					resourceType1.ResourceType,
					resourceType2.ResourceType,
					resourceType3.ResourceType,
					resourceTypeUsingBogusBaseType.ResourceType,
					resourceTypeOverridingBaseType.ResourceType,
				},
			},
			db.ConfigVersion(0),
			db.PipelineUnpaused,
		)
		Expect(err).ToNot(HaveOccurred())

		for _, rt := range []atc.VersionedResourceType{
			resourceType1,
			resourceType2,
			resourceType3,
			resourceTypeUsingBogusBaseType,
			resourceTypeOverridingBaseType,
		} {
			dbType, found, err := pipelineWithTypes.ResourceType("some-type")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			err = dbType.SaveVersion(rt.Version)
			Expect(err).NotTo(HaveOccurred())
		}

		build, err = defaultTeam.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())

		logger = lagertest.NewTestLogger("test")
	})

	Describe("FindOrCreateResourceCache", func() {
		It("creates resource cache in database", func() {
			usedResourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
				logger,
				db.ForBuild(build.ID()),
				"some-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				creds.NewVersionedResourceTypes(
					template.StaticVariables{"source-param": "some-secret-sauce"},
					atc.VersionedResourceTypes{
						resourceType1,
						resourceType2,
						resourceType3,
					},
				),
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(usedResourceCache.Version).To(Equal(atc.Version{"some": "version"}))

			rows, err := psql.Select("a.version, a.params_hash, o.source_hash, b.name").
				From("resource_caches a").
				LeftJoin("resource_configs o ON a.resource_config_id = o.id").
				LeftJoin("base_resource_types b ON o.base_resource_type_id = b.id").
				RunWith(dbConn).
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

			var toHash = func(s string) string {
				return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
			}

			Expect(resourceCaches).To(ConsistOf(
				resourceCache{
					Version:          `{"some-type-type":"version"}`,
					ParamsHash:       toHash(`{}`),
					BaseResourceName: "some-base-type",
					SourceHash:       toHash(`{"some-type-type":"some-secret-sauce"}`),
				},
				resourceCache{
					Version:    `{"some-type":"version"}`,
					ParamsHash: toHash(`{}`),
					SourceHash: toHash(`{"some-type":"source"}`),
				},
				resourceCache{
					Version:    `{"some":"version"}`,
					ParamsHash: toHash(`{"some":"params"}`),
					SourceHash: toHash(`{"some":"source"}`),
				},
			))
		})

		It("returns an error if base resource type does not exist", func() {
			_, err := resourceCacheFactory.FindOrCreateResourceCache(
				logger,
				db.ForBuild(build.ID()),
				"some-type-using-bogus-base-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				creds.NewVersionedResourceTypes(
					template.StaticVariables{"source-param": "some-secret-sauce"},
					atc.VersionedResourceTypes{
						resourceType1,
						resourceTypeUsingBogusBaseType,
					},
				),
			)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(db.ResourceTypeNotFoundError{Name: "some-bogus-base-type"}))
		})

		It("allows a base resource type to be overridden using itself", func() {
			usedResourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
				logger,
				db.ForBuild(build.ID()),
				"some-image-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				creds.NewVersionedResourceTypes(
					template.StaticVariables{"source-param": "some-secret-sauce"},
					atc.VersionedResourceTypes{
						resourceTypeOverridingBaseType,
					},
				),
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(usedResourceCache.Version).To(Equal(atc.Version{"some": "version"}))
			Expect(usedResourceCache.ResourceConfig.CreatedByResourceCache.Version).To(Equal(atc.Version{"some-image-type": "version"}))
			Expect(usedResourceCache.ResourceConfig.CreatedByResourceCache.ResourceConfig.CreatedByBaseResourceType.ID).To(Equal(usedImageBaseResourceType.ID))
		})

		Context("when the resource cache is concurrently deleted and created", func() {
			BeforeEach(func() {
				Expect(build.Finish(db.BuildStatusSucceeded)).To(Succeed())
				Expect(build.SetInterceptible(false)).To(Succeed())
			})

			It("consistently is able to be used", func() {
				// enable concurrent use of database. this is set to 1 by default to
				// ensure methods don't require more than one in a single connection,
				// which can cause deadlocking as the pool is limited.
				dbConn.SetMaxOpenConns(10)

				done := make(chan struct{})

				wg := new(sync.WaitGroup)
				for i := 0; i < 5; i++ {
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()

						for {
							select {
							case <-done:
								return
							default:
								Expect(resourceCacheLifecycle.CleanUsesForFinishedBuilds(logger)).To(Succeed())
								Expect(resourceCacheLifecycle.CleanUpInvalidCaches(logger)).To(Succeed())
								Expect(resourceConfigFactory.CleanUnreferencedConfigs()).To(Succeed())
							}
						}
					}()
				}

				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer close(done)
					defer wg.Done()

					for i := 0; i < 100; i++ {
						_, err := resourceCacheFactory.FindOrCreateResourceCache(
							logger,
							db.ForBuild(build.ID()),
							"some-base-resource-type",
							atc.Version{"some": "version"},
							atc.Source{"some": "source"},
							atc.Params{"some": "params"},
							creds.VersionedResourceTypes{},
						)
						Expect(err).ToNot(HaveOccurred())
					}
				}()

				wg.Wait()
			})
		})
	})

})

type resourceCache struct {
	Version          string
	ParamsHash       string
	SourceHash       string
	BaseResourceName string
}
