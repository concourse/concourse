package db_test

import (
	"context"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
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
				atc.VersionedResourceTypes{},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(urc.ID()).ToNot(BeZero())

			// ON DELETE RESTRICT from resource_cache_uses -> resource_caches
			_, err = psql.Delete("resource_caches").Where(sq.Eq{"id": urc.ID()}).RunWith(dbConn).Exec()
			Expect(err).To(HaveOccurred())
			Expect(err.(*pq.Error).Code.Name()).To(Equal("foreign_key_violation"))
		})

		Context("when it already exists", func() {
			var existingResourceCache db.UsedResourceCache

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
					atc.VersionedResourceTypes{},
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
					atc.VersionedResourceTypes{},
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID()).To(Equal(existingResourceCache.ID()))
			})
		})
	})

	Describe("creating for a container", func() {
		var container db.CreatingContainer
		var urc db.UsedResourceCache

		BeforeEach(func() {
			worker, err := defaultTeam.SaveWorker(atc.Worker{
				Name: "some-worker",
			}, 0)
			Expect(err).ToNot(HaveOccurred())

			build, err := defaultTeam.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			container, err = worker.CreateContainer(
				db.NewBuildStepContainerOwner(build.ID(), "some-plan", defaultTeam.ID()),
				db.ContainerMetadata{},
			)
			Expect(err).ToNot(HaveOccurred())

			urc, err = resourceCacheFactory.FindOrCreateResourceCache(
				db.ForContainer(container.ID()),
				"some-worker-resource-type",
				atc.Version{"some-type": "version"},
				atc.Source{
					"cache": "source",
				},
				atc.Params{"some": "params"},
				atc.VersionedResourceTypes{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("resource cache cannot be deleted through use", func() {
			var err error
			// ON DELETE RESTRICT from resource_cache_uses -> resource_caches
			_, err = psql.Delete("resource_caches").Where(sq.Eq{"id": urc.ID()}).RunWith(dbConn).Exec()
			Expect(err).To(HaveOccurred())
			Expect(err.(*pq.Error).Code.Name()).To(Equal("foreign_key_violation"))
		})

		Context("when it already exists", func() {
			var existingResourceCache db.UsedResourceCache

			BeforeEach(func() {
				var err error
				existingResourceCache, err = resourceCacheFactory.FindOrCreateResourceCache(
					db.ForContainer(container.ID()),
					"some-worker-resource-type",
					atc.Version{"some-type": "version"},
					atc.Source{
						"cache": "source",
					},
					atc.Params{"some": "params"},
					atc.VersionedResourceTypes{},
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the same used resource cache", func() {
				Expect(urc.ID()).To(Equal(existingResourceCache.ID()))
			})
		})
	})

	Describe("version and metadata", func() {
		var urc db.UsedResourceCache
		var expectedMetadata = db.ResourceConfigMetadataFields{
			db.ResourceConfigMetadataField{
				Name:  "n1",
				Value: "v1",
			},
			db.ResourceConfigMetadataField{
				Name:  "n2",
				Value: "v2",
			},
		}

		BeforeEach(func() {
			build, err := defaultTeam.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			urc, err = resourceCacheFactory.FindOrCreateResourceCache(
				db.ForBuild(build.ID()),
				"some-worker-resource-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				atc.VersionedResourceTypes{},
			)
			Expect(err).ToNot(HaveOccurred())

			rcs, err := urc.ResourceConfig().FindOrCreateScope(defaultResource)
			Expect(err).ToNot(HaveOccurred())

			err = defaultResource.SetResourceConfigScope(rcs)
			Expect(err).ToNot(HaveOccurred())
			_, err = defaultResource.Reload()
			Expect(err).ToNot(HaveOccurred())

			err = rcs.SaveVersions(db.NewSpanContext(context.Background()), []atc.Version{urc.Version()})
			Expect(err).ToNot(HaveOccurred())

			err = defaultResource.SetResourceConfigScope(rcs)
			Expect(err).ToNot(HaveOccurred())

			found, err := defaultResource.UpdateMetadata(urc.Version(), expectedMetadata)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("should load the metadata", func() {
			metadata, err := urc.LoadVersionMetadata()
			Expect(err).ToNot(HaveOccurred())
			Expect(metadata).To(Equal(expectedMetadata.ToATCMetadata()))
		})
	})
})
