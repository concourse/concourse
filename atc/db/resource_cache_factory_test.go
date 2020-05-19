package db_test

import (
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"

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
		err    error
	)

	BeforeEach(func() {
		build, err = defaultTeam.CreateOneOffBuild()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("FindOrCreateResourceCache", func() {
		BeforeEach(func() {
			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			baseResourceType := db.BaseResourceType{
				Name: "some-base-type",
			}

			_, err = baseResourceType.FindOrCreate(setupTx, false)
			Expect(err).NotTo(HaveOccurred())

			imageBaseResourceType := db.BaseResourceType{
				Name: "some-image-type",
			}

			resourceCacheLifecycle = db.NewResourceCacheLifecycle(dbConn)

			usedImageBaseResourceType, err = imageBaseResourceType.FindOrCreate(setupTx, false)
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
						"some-type-type": "some-secret-sauce",
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

			logger = lagertest.NewTestLogger("test")
		})
		It("creates resource cache in database", func() {
			usedResourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
				db.ForBuild(build.ID()),
				"some-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				atc.VersionedResourceTypes{
					resourceType1,
					resourceType2,
					resourceType3,
				},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(usedResourceCache.Version()).To(Equal(atc.Version{"some": "version"}))

			rows, err := psql.Select("a.version, a.version_md5, a.params_hash, o.source_hash, b.name").
				From("resource_caches a").
				LeftJoin("resource_configs o ON a.resource_config_id = o.id").
				LeftJoin("base_resource_types b ON o.base_resource_type_id = b.id").
				RunWith(dbConn).
				Query()
			Expect(err).NotTo(HaveOccurred())
			resourceCaches := []resourceCache{}
			for rows.Next() {
				var version string
				var versionMd5 string
				var paramsHash string
				var sourceHash sql.NullString
				var baseResourceTypeName sql.NullString

				err := rows.Scan(&version, &versionMd5, &paramsHash, &sourceHash, &baseResourceTypeName)
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
					VersionMd5:       versionMd5,
					ParamsHash:       paramsHash,
					SourceHash:       sourceHashString,
					BaseResourceName: baseResourceTypeNameString,
				})
			}

			var toHash = func(s string) string {
				return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
			}

			var toMd5 = func(s string) string {
				return fmt.Sprintf("%x", md5.Sum([]byte(s)))
			}

			Expect(resourceCaches).To(ConsistOf(
				resourceCache{
					Version:          `{"some-type-type": "version"}`,
					VersionMd5:       toMd5(`{"some-type-type":"version"}`),
					ParamsHash:       toHash(`{}`),
					BaseResourceName: "some-base-type",
					SourceHash:       toHash(`{"some-type-type":"some-secret-sauce"}`),
				},
				resourceCache{
					Version:    `{"some-type": "version"}`,
					VersionMd5: toMd5(`{"some-type":"version"}`),
					ParamsHash: toHash(`{}`),
					SourceHash: toHash(`{"some-type":"source"}`),
				},
				resourceCache{
					Version:    `{"some": "version"}`,
					VersionMd5: toMd5(`{"some":"version"}`),
					ParamsHash: toHash(`{"some":"params"}`),
					SourceHash: toHash(`{"some":"source"}`),
				},
			))
		})

		It("returns an error if base resource type does not exist", func() {
			_, err := resourceCacheFactory.FindOrCreateResourceCache(
				db.ForBuild(build.ID()),
				"some-type-using-bogus-base-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				atc.VersionedResourceTypes{
					resourceType1,
					resourceTypeUsingBogusBaseType,
				},
			)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(db.BaseResourceTypeNotFoundError{Name: "some-bogus-base-type"}))
		})

		It("allows a base resource type to be overridden using itself", func() {
			usedResourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
				db.ForBuild(build.ID()),
				"some-image-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				atc.VersionedResourceTypes{
					resourceTypeOverridingBaseType,
				},
			)
			Expect(err).ToNot(HaveOccurred())

			Expect(usedResourceCache.ResourceConfig().CreatedByResourceCache().ResourceConfig().CreatedByBaseResourceType().ID).To(Equal(usedImageBaseResourceType.ID))
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
							db.ForBuild(build.ID()),
							"some-base-resource-type",
							atc.Version{"some": "version"},
							atc.Source{"some": "source"},
							atc.Params{"some": "params"},
							atc.VersionedResourceTypes{},
						)
						Expect(err).ToNot(HaveOccurred())
					}
				}()

				wg.Wait()
			})
		})
	})

	Describe("FindResourceCacheByID", func() {
		var resourceCacheUser db.ResourceCacheUser
		var someUsedResourceCacheFromBaseResource db.UsedResourceCache
		var someUsedResourceCacheFromCustomResource db.UsedResourceCache
		BeforeEach(func() {
			resourceCacheUser = db.ForBuild(build.ID())

			someUsedResourceCacheFromBaseResource, err = resourceCacheFactory.FindOrCreateResourceCache(resourceCacheUser,
				"some-base-resource-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": fmt.Sprintf("param-%d", time.Now().UnixNano())},
				atc.VersionedResourceTypes{},
			)
			Expect(err).ToNot(HaveOccurred())

			someUsedResourceCacheFromCustomResource, err = resourceCacheFactory.FindOrCreateResourceCache(resourceCacheUser,
				"some-custom-resource-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": fmt.Sprintf("param-%d", time.Now().UnixNano())},
				atc.VersionedResourceTypes{
					atc.VersionedResourceType{
						ResourceType: atc.ResourceType{
							Name: "some-custom-resource-type",
							Type: "some-base-resource-type",
							Source: atc.Source{
								"some": "source",
							},
						},
						Version: atc.Version{"showme": "whatyougot"},
					},
				},
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns a UsedResourceCache from a BaseResource", func() {
			actualUsedResourceCache, found, err := resourceCacheFactory.FindResourceCacheByID(someUsedResourceCacheFromBaseResource.ID())

			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
			Expect(actualUsedResourceCache).To(Equal(someUsedResourceCacheFromBaseResource))
			Expect(actualUsedResourceCache.ResourceConfig().CreatedByBaseResourceType().Name).To(Equal("some-base-resource-type"))
			Expect(actualUsedResourceCache.ResourceConfig().CreatedByResourceCache()).To(BeNil())
		})

		It("returns a UsedResourceCache from a custom ResourceCache", func() {
			actualUsedResourceCache, found, err := resourceCacheFactory.FindResourceCacheByID(someUsedResourceCacheFromCustomResource.ID())

			Expect(found).To(BeTrue())
			Expect(err).ToNot(HaveOccurred())
			Expect(actualUsedResourceCache).To(Equal(someUsedResourceCacheFromCustomResource))
			Expect(actualUsedResourceCache.ResourceConfig().CreatedByResourceCache().Version()).To(Equal(atc.Version{"showme": "whatyougot"}))
		})

		It("returns !found when one is not found", func() {
			_, found, err := resourceCacheFactory.FindResourceCacheByID(42)

			Expect(found).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
		})
	})

})

type resourceCache struct {
	Version          string
	VersionMd5       string
	ParamsHash       string
	SourceHash       string
	BaseResourceName string
}
