package db_test

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/lib/pq"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceCache", func() {
	var (
		resourceCacheFactory db.ResourceCacheFactory
	)

	BeforeEach(func() {
		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		brt := db.BaseResourceType{
			Name: "some-worker-resource-type",
		}

		_, err = brt.FindOrCreate(setupTx, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(setupTx.Commit()).To(Succeed())

		resourceCacheFactory = db.NewResourceCacheFactory(dbConn, lockFactory)
	})

	Describe("creating for a build", func() {
		var build db.Build

		BeforeEach(func() {
			var err error
			build, err = defaultTeam.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())
		})

		It("can be created and used", func() {
			urc, err := resourceCacheFactory.FindOrCreateResourceCache(
				db.ForBuild(build.ID()),
				"some-worker-resource-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				nil,
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(urc.ID()).ToNot(BeZero())

			// ON DELETE RESTRICT from resource_cache_uses -> resource_caches
			_, err = psql.Delete("resource_caches").Where(sq.Eq{"id": urc.ID()}).RunWith(dbConn).Exec()
			Expect(err).To(HaveOccurred())
			Expect(err.(*pq.Error).Code.Name()).To(Equal("foreign_key_violation"))
		})

		Context("when it already exists", func() {
			var existingResourceCache db.ResourceCache

			BeforeEach(func() {
				var err error
				existingResourceCache, err = resourceCacheFactory.FindOrCreateResourceCache(
					db.ForBuild(build.ID()),
					"some-worker-resource-type",
					atc.Version{"some": "version"},
					atc.Source{
						"some": "source",
					},
					atc.Params{"some": "params"},
					nil,
				)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the same used resource cache", func() {
				urc, err := resourceCacheFactory.FindOrCreateResourceCache(
					db.ForBuild(build.ID()),
					"some-worker-resource-type",
					atc.Version{"some": "version"},
					atc.Source{
						"some": "source",
					},
					atc.Params{"some": "params"},
					nil,
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID()).To(Equal(existingResourceCache.ID()))
			})
		})
	})

	Describe("creating for in-memory-build", func() {
		It("can be created and used", func() {
			urc, err := resourceCacheFactory.FindOrCreateResourceCache(
				db.ForInMemoryBuild(99, time.Now()),
				"some-worker-resource-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				nil,
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(urc.ID()).ToNot(BeZero())

			// ON DELETE RESTRICT from resource_cache_uses -> resource_caches
			_, err = psql.Delete("resource_caches").Where(sq.Eq{"id": urc.ID()}).RunWith(dbConn).Exec()
			Expect(err).To(HaveOccurred())
			Expect(err.(*pq.Error).Code.Name()).To(Equal("foreign_key_violation"))
		})
	})
})
