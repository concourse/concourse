package db_test

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceCache", func() {
	var (
		tx db.Tx

		cache                db.ResourceCache
		resourceCacheFactory db.ResourceCacheFactory
	)

	BeforeEach(func() {
		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		brt := db.BaseResourceType{
			Name: "some-worker-resource-type",
		}
		_, err = brt.FindOrCreate(setupTx)
		Expect(err).NotTo(HaveOccurred())
		Expect(setupTx.Commit()).To(Succeed())

		cache = db.ResourceCache{
			ResourceConfig: db.ResourceConfig{
				CreatedByBaseResourceType: &brt,

				Source: atc.Source{"some": "source"},
			},
			Version: atc.Version{"some": "version"},
			Params:  atc.Params{"some": "params"},
		}
		resourceCacheFactory = db.NewResourceCacheFactory(dbConn)
	})

	Describe("creating for a build", func() {
		var build db.Build

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
			urc, err := db.ForBuild(build.ID()).UseResourceCache(logger, tx, cache)
			Expect(err).ToNot(HaveOccurred())
			Expect(urc.ID).ToNot(BeZero())

			// ON DELETE RESTRICT from resource_cache_uses -> resource_caches
			_, err = psql.Delete("resource_caches").Where(sq.Eq{"id": urc.ID}).RunWith(tx).Exec()
			Expect(err).To(HaveOccurred())
			Expect(err.(*pq.Error).Code.Name()).To(Equal("foreign_key_violation"))
		})

		Context("when it already exists", func() {
			var existingResourceCache *db.UsedResourceCache

			BeforeEach(func() {
				var err error
				existingResourceCache, err = db.ForBuild(build.ID()).UseResourceCache(logger, tx, cache)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the same used resource cache", func() {
				urc, err := db.ForBuild(build.ID()).UseResourceCache(logger, tx, cache)
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID).To(Equal(existingResourceCache.ID))
			})
		})
	})

	Describe("creating for a resource", func() {
		var resource db.Resource

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
			urc, err := db.ForResource(resource.ID()).UseResourceCache(logger, tx, cache)
			Expect(err).ToNot(HaveOccurred())
			Expect(urc.ID).ToNot(BeZero())

			// ON DELETE RESTRICT from resource_cache_uses -> resource_caches
			_, err = psql.Delete("resource_caches").Where(sq.Eq{"id": urc.ID}).RunWith(tx).Exec()
			Expect(err).To(HaveOccurred())
			Expect(err.(*pq.Error).Code.Name()).To(Equal("foreign_key_violation"))
		})

		Context("when it already exists", func() {
			var existingResourceCache *db.UsedResourceCache

			BeforeEach(func() {
				var err error
				existingResourceCache, err = db.ForResource(resource.ID()).UseResourceCache(logger, tx, cache)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the same used resource cache", func() {
				urc, err := db.ForResource(resource.ID()).UseResourceCache(logger, tx, cache)
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID).To(Equal(existingResourceCache.ID))
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
			urc, err := db.ForResourceType(defaultResourceType.ID()).UseResourceCache(logger, tx, cache)
			Expect(err).ToNot(HaveOccurred())
			Expect(urc.ID).ToNot(BeZero())

			// ON DELETE RESTRICT from resource_cache_uses -> resource_caches
			_, err = psql.Delete("resource_caches").Where(sq.Eq{"id": urc.ID}).RunWith(tx).Exec()
			Expect(err).To(HaveOccurred())
			Expect(err.(*pq.Error).Code.Name()).To(Equal("foreign_key_violation"))
		})

		Context("when it already exists", func() {
			var existingResourceCache *db.UsedResourceCache

			BeforeEach(func() {
				var err error
				existingResourceCache, err = db.ForResourceType(defaultResourceType.ID()).UseResourceCache(logger, tx, cache)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the same used resource cache", func() {
				urc, err := db.ForResourceType(defaultResourceType.ID()).UseResourceCache(logger, tx, cache)
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID).To(Equal(existingResourceCache.ID))
			})
		})

		Context("when it already exists but with different params", func() {
			var existingResourceCache *db.UsedResourceCache

			BeforeEach(func() {
				var err error
				existingResourceCache, err = db.ForResourceType(defaultResourceType.ID()).UseResourceCache(logger, tx, cache)
				Expect(err).ToNot(HaveOccurred())
			})

			It("creates it, and does not use the existing one [#139960779]", func() {
				cache.Params = atc.Params{
					"foo": "bar",
				}

				urc, err := db.ForResourceType(defaultResourceType.ID()).UseResourceCache(logger, tx, cache)
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID).NotTo(Equal(existingResourceCache.ID))
			})
		})
	})
})
