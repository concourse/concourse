package db_test

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
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
		_, err = brt.FindOrCreate(setupTx)
		Expect(err).NotTo(HaveOccurred())
		Expect(setupTx.Commit()).To(Succeed())

		resourceCacheFactory = db.NewResourceCacheFactory(dbConn)
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
				logger,
				db.ForBuild(build.ID()),
				"some-worker-resource-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				creds.NewVersionedResourceTypes(
					template.StaticVariables{"source-param": "some-secret-sauce"},
					atc.VersionedResourceTypes{},
				),
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(urc.ID).ToNot(BeZero())

			// ON DELETE RESTRICT from resource_cache_uses -> resource_caches
			_, err = psql.Delete("resource_caches").Where(sq.Eq{"id": urc.ID}).RunWith(dbConn).Exec()
			Expect(err).To(HaveOccurred())
			Expect(err.(*pq.Error).Code.Name()).To(Equal("foreign_key_violation"))
		})

		Context("when it already exists", func() {
			var existingResourceCache *db.UsedResourceCache

			BeforeEach(func() {
				var err error
				existingResourceCache, err = resourceCacheFactory.FindOrCreateResourceCache(
					logger,
					db.ForBuild(build.ID()),
					"some-worker-resource-type",
					atc.Version{"some": "version"},
					atc.Source{
						"some": "source",
					},
					atc.Params{"some": "params"},
					creds.NewVersionedResourceTypes(
						template.StaticVariables{"source-param": "some-secret-sauce"},
						atc.VersionedResourceTypes{},
					),
				)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the same used resource cache", func() {
				urc, err := resourceCacheFactory.FindOrCreateResourceCache(
					logger,
					db.ForBuild(build.ID()),
					"some-worker-resource-type",
					atc.Version{"some": "version"},
					atc.Source{
						"some": "source",
					},
					atc.Params{"some": "params"},
					creds.NewVersionedResourceTypes(
						template.StaticVariables{"source-param": "some-secret-sauce"},
						atc.VersionedResourceTypes{},
					),
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(urc.ID).To(Equal(existingResourceCache.ID))
			})
		})
	})

	Describe("creating for a container", func() {
		var container db.CreatingContainer
		var urc *db.UsedResourceCache

		BeforeEach(func() {
			worker, err := defaultTeam.SaveWorker(atc.Worker{
				Name: "some-worker",
			}, 0)
			Expect(err).ToNot(HaveOccurred())

			build, err := defaultTeam.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			container, err = defaultTeam.CreateContainer(
				worker.Name(),
				db.NewBuildStepContainerOwner(build.ID(), "some-plan"),
				db.ContainerMetadata{},
			)
			Expect(err).ToNot(HaveOccurred())

			urc, err = resourceCacheFactory.FindOrCreateResourceCache(
				logger,
				db.ForContainer(container.ID()),
				"some-worker-resource-type",
				atc.Version{"some-type": "version"},
				atc.Source{
					"cache": "source",
				},
				atc.Params{"some": "params"},
				creds.NewVersionedResourceTypes(template.StaticVariables{"source-param": "some-secret-sauce"},
					atc.VersionedResourceTypes{},
				),
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("resource cache cannot be deleted through use", func() {
			var err error
			// ON DELETE RESTRICT from resource_cache_uses -> resource_caches
			_, err = psql.Delete("resource_caches").Where(sq.Eq{"id": urc.ID}).RunWith(dbConn).Exec()
			Expect(err).To(HaveOccurred())
			Expect(err.(*pq.Error).Code.Name()).To(Equal("foreign_key_violation"))
		})

		Context("when it already exists", func() {
			var existingResourceCache *db.UsedResourceCache

			BeforeEach(func() {
				var err error
				existingResourceCache, err = resourceCacheFactory.FindOrCreateResourceCache(
					logger,
					db.ForContainer(container.ID()),
					"some-worker-resource-type",
					atc.Version{"some-type": "version"},
					atc.Source{
						"cache": "source",
					},
					atc.Params{"some": "params"},
					creds.NewVersionedResourceTypes(template.StaticVariables{"source-param": "some-secret-sauce"},
						atc.VersionedResourceTypes{},
					),
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the same used resource cache", func() {
				Expect(urc.ID).To(Equal(existingResourceCache.ID))
			})
		})
	})
})
