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
	var tx dbng.Tx

	var cache dbng.ResourceCache

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
	})

	Describe("creating for a build", func() {
		var build dbng.Build

		BeforeEach(func() {
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
			urc, err := cache.FindOrCreateForBuild(logger, tx, lockFactory, build.ID())
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
				existingResourceCache, err = cache.FindOrCreateForBuild(logger, tx, lockFactory, build.ID())
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the same used resource cache", func() {
				urc, err := cache.FindOrCreateForBuild(logger, tx, lockFactory, build.ID())
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID).To(Equal(existingResourceCache.ID))
			})
		})
	})

	Describe("creating for a resource", func() {
		var resource *dbng.Resource

		BeforeEach(func() {
			resource, err = defaultPipeline.CreateResource("some-resource", atc.ResourceConfig{})
			Expect(err).ToNot(HaveOccurred())

			tx, err = dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := tx.Rollback()
			Expect(err).NotTo(HaveOccurred())
		})

		It("can be created and used", func() {
			urc, err := cache.FindOrCreateForResource(logger, tx, lockFactory, resource.ID)
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
				existingResourceCache, err = cache.FindOrCreateForResource(logger, tx, lockFactory, resource.ID)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the same used resource cache", func() {
				urc, err := cache.FindOrCreateForResource(logger, tx, lockFactory, resource.ID)
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID).To(Equal(existingResourceCache.ID))
			})
		})
	})

	Describe("creating for a resource type", func() {
		var resourceType *dbng.UsedResourceType

		BeforeEach(func() {
			rf := dbng.NewResourceTypeFactory(dbConn)
			resourceType, err = rf.CreateResourceType(
				defaultPipeline.ID(),
				atc.ResourceType{
					Name: "some-resource-type",
					Type: "some-resource-type-type",
				},
				atc.Version{},
			)
			Expect(err).ToNot(HaveOccurred())

			tx, err = dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := tx.Rollback()
			Expect(err).NotTo(HaveOccurred())
		})

		It("can be created and used", func() {
			urc, err := cache.FindOrCreateForResourceType(logger, tx, lockFactory, resourceType)
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
				existingResourceCache, err = cache.FindOrCreateForResourceType(logger, tx, lockFactory, resourceType)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the same used resource cache", func() {
				urc, err := cache.FindOrCreateForResourceType(logger, tx, lockFactory, resourceType)
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID).To(Equal(existingResourceCache.ID))
			})
		})

		Context("when it already exists but with different params", func() {
			var existingResourceCache *dbng.UsedResourceCache

			BeforeEach(func() {
				var err error
				existingResourceCache, err = cache.FindOrCreateForResourceType(logger, tx, lockFactory, resourceType)
				Expect(err).ToNot(HaveOccurred())
			})

			It("creates it, and does not use the existing one [#139960779]", func() {
				cache.Params = atc.Params{
					"foo": "bar",
				}

				urc, err := cache.FindOrCreateForResourceType(logger, tx, lockFactory, resourceType)
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID).NotTo(Equal(existingResourceCache.ID))
			})
		})
	})

})
