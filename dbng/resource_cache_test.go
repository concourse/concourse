package dbng_test

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceCache", func() {
	var (
		tx dbng.Tx

		cache                dbng.ResourceCache
		resourceCacheFactory dbng.ResourceCacheFactory
	)

	BeforeEach(func() {
		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		brt := dbng.BaseResourceType{
			Name: "some-worker-resource-type",
		}
		_, err = brt.FindOrCreate(setupTx)
		Expect(err).NotTo(HaveOccurred())
		Expect(setupTx.Commit()).To(Succeed())

		cache = dbng.ResourceCache{
			ResourceConfig: dbng.ResourceConfig{
				CreatedByBaseResourceType: &brt,

				Source: atc.Source{"some": "source"},
			},
			Version: atc.Version{"some": "version"},
			Params:  atc.Params{"some": "params"},
		}
		resourceCacheFactory = dbng.NewResourceCacheFactory(dbConn, lockFactory)
	})

	Describe("creating for a build", func() {
		var build dbng.Build

		BeforeEach(func() {
			var err error
			build, err = defaultTeam.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			tx, err = dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := tx.Rollback()
			Expect(err).NotTo(HaveOccurred())
		})

		It("can be created and used", func() {
			urc, err := dbng.ForBuild(build.ID()).UseResourceCache(logger, tx, lockFactory, cache)
			Expect(err).ToNot(HaveOccurred())
			Expect(urc.ID).ToNot(BeZero())

			// ON DELETE RESTRICT from resource_cache_uses -> resource_caches
			_, err = psql.Delete("resource_caches").Where(sq.Eq{"id": urc.ID}).RunWith(tx).Exec()
			Expect(err).To(HaveOccurred())
			Expect(err.(*pq.Error).Code.Name()).To(Equal("foreign_key_violation"))
		})

		Context("when it already exists", func() {
			var existingResourceCache *dbng.UsedResourceCache

			BeforeEach(func() {
				var err error
				existingResourceCache, err = dbng.ForBuild(build.ID()).UseResourceCache(logger, tx, lockFactory, cache)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the same used resource cache", func() {
				urc, err := dbng.ForBuild(build.ID()).UseResourceCache(logger, tx, lockFactory, cache)
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID).To(Equal(existingResourceCache.ID))
			})

			Context("when it has metadata", func() {
				BeforeEach(func() {
					Expect(tx.Commit()).To(Succeed())
					err := resourceCacheFactory.UpdateResourceCacheMetadata(existingResourceCache, []atc.MetadataField{
						{Name: "some", Value: "metadata"},
					})
					Expect(err).ToNot(HaveOccurred())
					tx, err = dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns resource cache with metadata", func() {
					urc, err := dbng.ForBuild(build.ID()).UseResourceCache(logger, tx, lockFactory, cache)
					Expect(err).ToNot(HaveOccurred())
					Expect(urc.ID).To(Equal(existingResourceCache.ID))
					Expect(urc.Metadata).To(Equal(dbng.ResourceMetadataFields{
						{Name: "some", Value: "metadata"},
					}))
				})
			})
		})
	})

	Describe("creating for a resource", func() {
		var resource dbng.Resource

		BeforeEach(func() {
			var found bool
			var err error
			resource, found, err = defaultPipeline.Resource("some-resource")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			tx, err = dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := tx.Rollback()
			Expect(err).NotTo(HaveOccurred())
		})

		It("can be created and used", func() {
			urc, err := dbng.ForResource(resource.ID()).UseResourceCache(logger, tx, lockFactory, cache)
			Expect(err).ToNot(HaveOccurred())
			Expect(urc.ID).ToNot(BeZero())

			// ON DELETE RESTRICT from resource_cache_uses -> resource_caches
			_, err = psql.Delete("resource_caches").Where(sq.Eq{"id": urc.ID}).RunWith(tx).Exec()
			Expect(err).To(HaveOccurred())
			Expect(err.(*pq.Error).Code.Name()).To(Equal("foreign_key_violation"))
		})

		Context("when it already exists", func() {
			var existingResourceCache *dbng.UsedResourceCache

			BeforeEach(func() {
				var err error
				existingResourceCache, err = dbng.ForResource(resource.ID()).UseResourceCache(logger, tx, lockFactory, cache)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the same used resource cache", func() {
				urc, err := dbng.ForResource(resource.ID()).UseResourceCache(logger, tx, lockFactory, cache)
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID).To(Equal(existingResourceCache.ID))
			})

			Context("when it has metadata", func() {
				BeforeEach(func() {
					Expect(tx.Commit()).To(Succeed())
					err := resourceCacheFactory.UpdateResourceCacheMetadata(existingResourceCache, []atc.MetadataField{
						{Name: "some", Value: "metadata"},
					})
					Expect(err).ToNot(HaveOccurred())
					tx, err = dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns resource cache with metadata", func() {
					urc, err := dbng.ForResource(resource.ID()).UseResourceCache(logger, tx, lockFactory, cache)
					Expect(err).ToNot(HaveOccurred())
					Expect(urc.ID).To(Equal(existingResourceCache.ID))
					Expect(urc.Metadata).To(Equal(dbng.ResourceMetadataFields{
						{Name: "some", Value: "metadata"},
					}))
				})
			})
		})
	})

	Describe("creating for a resource type", func() {
		BeforeEach(func() {
			var err error
			tx, err = dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := tx.Rollback()
			Expect(err).NotTo(HaveOccurred())
		})

		It("can be created and used", func() {
			urc, err := dbng.ForResourceType(defaultResourceType.ID()).UseResourceCache(logger, tx, lockFactory, cache)
			Expect(err).ToNot(HaveOccurred())
			Expect(urc.ID).ToNot(BeZero())

			// ON DELETE RESTRICT from resource_cache_uses -> resource_caches
			_, err = psql.Delete("resource_caches").Where(sq.Eq{"id": urc.ID}).RunWith(tx).Exec()
			Expect(err).To(HaveOccurred())
			Expect(err.(*pq.Error).Code.Name()).To(Equal("foreign_key_violation"))
		})

		Context("when it already exists", func() {
			var existingResourceCache *dbng.UsedResourceCache

			BeforeEach(func() {
				var err error
				existingResourceCache, err = dbng.ForResourceType(defaultResourceType.ID()).UseResourceCache(logger, tx, lockFactory, cache)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the same used resource cache", func() {
				urc, err := dbng.ForResourceType(defaultResourceType.ID()).UseResourceCache(logger, tx, lockFactory, cache)
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID).To(Equal(existingResourceCache.ID))
			})

			Context("when it has metadata", func() {
				BeforeEach(func() {
					Expect(tx.Commit()).To(Succeed())
					err := resourceCacheFactory.UpdateResourceCacheMetadata(existingResourceCache, []atc.MetadataField{
						{Name: "some", Value: "metadata"},
					})
					Expect(err).ToNot(HaveOccurred())
					tx, err = dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns resource cache with metadata", func() {
					urc, err := dbng.ForResourceType(defaultResourceType.ID()).UseResourceCache(logger, tx, lockFactory, cache)
					Expect(err).ToNot(HaveOccurred())
					Expect(urc.ID).To(Equal(existingResourceCache.ID))
					Expect(urc.Metadata).To(Equal(dbng.ResourceMetadataFields{
						{Name: "some", Value: "metadata"},
					}))
				})
			})
		})

		Context("when it already exists but with different params", func() {
			var existingResourceCache *dbng.UsedResourceCache

			BeforeEach(func() {
				var err error
				existingResourceCache, err = dbng.ForResourceType(defaultResourceType.ID()).UseResourceCache(logger, tx, lockFactory, cache)
				Expect(err).ToNot(HaveOccurred())
			})

			It("creates it, and does not use the existing one [#139960779]", func() {
				cache.Params = atc.Params{
					"foo": "bar",
				}

				urc, err := dbng.ForResourceType(defaultResourceType.ID()).UseResourceCache(logger, tx, lockFactory, cache)
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID).NotTo(Equal(existingResourceCache.ID))
			})
		})
	})

})
