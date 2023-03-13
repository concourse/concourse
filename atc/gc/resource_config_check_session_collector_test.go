package gc_test

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"
	"github.com/concourse/concourse/atc/gc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfigCheckSessionCollector", func() {
	var (
		collector                           GcCollector
		resourceConfigCheckSessionLifecycle db.ResourceConfigCheckSessionLifecycle
		ownerExpiries                       db.ContainerOwnerExpiries
	)

	BeforeEach(func() {
		resourceConfigCheckSessionLifecycle = db.NewResourceConfigCheckSessionLifecycle(dbConn)
		resourceConfigFactory = db.NewResourceConfigFactory(dbConn, lockFactory)
		collector = gc.NewResourceConfigCheckSessionCollector(resourceConfigCheckSessionLifecycle)
	})

	Describe("Run", func() {
		var scenario *dbtest.Scenario
		var resourceConfig db.ResourceConfig

		var owner db.ContainerOwner

		ownerExpiries = db.ContainerOwnerExpiries{
			Min: 10 * time.Second,
			Max: 10 * time.Second,
		}

		BeforeEach(func() {
			scenario = dbtest.Setup(
				builder.WithBaseWorker(),
				builder.WithPipeline(atc.Config{
					Resources: atc.ResourceConfigs{
						{
							Name:   "some-resource",
							Type:   dbtest.BaseResourceType,
							Source: atc.Source{"some": "source"},
						},
					},
				}),
				builder.WithResourceVersions("some-resource", atc.Version{"some": "version"}),
			)

			resource := scenario.Resource("some-resource")

			var found bool
			var err error
			resourceConfig, found, err = resourceConfigFactory.FindResourceConfigByID(resource.ResourceConfigID())
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			owner = db.NewResourceConfigCheckSessionContainerOwner(
				resourceConfig.ID(),
				resourceConfig.OriginBaseResourceType().ID,
				ownerExpiries,
			)

			_, err = scenario.Workers[0].CreateContainer(owner, db.ContainerMetadata{})
			Expect(err).NotTo(HaveOccurred())
		})

		resourceConfigCheckSessionExists := func(resourceConfig db.ResourceConfig) bool {
			var count int
			err = psql.Select("COUNT(*)").
				From("resource_config_check_sessions").
				Where(sq.Eq{"resource_config_id": resourceConfig.ID()}).
				RunWith(dbConn).
				QueryRow().
				Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			return count == 1
		}

		JustBeforeEach(func() {
			Expect(collector.Run(context.TODO())).To(Succeed())
		})

		Context("when the resource is active", func() {
			It("keeps the resource config check session", func() {
				Expect(resourceConfigCheckSessionExists(resourceConfig)).To(BeTrue())
			})
		})

		Context("when the resource config check session is expired", func() {
			BeforeEach(func() {
				res, err := psql.Update("resource_config_check_sessions").
					Set("expires_at", sq.Expr("now()")).
					Where(sq.Eq{"resource_config_id": resourceConfig.ID()}).
					RunWith(dbConn).
					Exec()
				Expect(err).ToNot(HaveOccurred())

				rows, err := res.RowsAffected()
				Expect(err).ToNot(HaveOccurred())
				Expect(rows).To(Equal(int64(1)))
			})

			It("removes the resource config check session", func() {
				Expect(resourceConfigCheckSessionExists(resourceConfig)).To(BeFalse())
			})
		})

		Context("when the resource config changes", func() {
			BeforeEach(func() {
				scenario.Run(
					builder.WithPipeline(atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name:   "some-resource",
								Type:   dbtest.BaseResourceType,
								Source: atc.Source{"some": "different-source"},
							},
						},
					}),
					builder.WithResourceVersions("some-resource", atc.Version{"some": "different-version"}),
				)
			})

			It("removes the resource config check session", func() {
				Expect(resourceConfigCheckSessionExists(resourceConfig)).To(BeFalse())
			})
		})

		Context("when the resource is removed", func() {
			BeforeEach(func() {
				scenario.Run(
					builder.WithPipeline(atc.Config{}),
				)
			})

			It("removes the resource config check session", func() {
				Expect(resourceConfigCheckSessionExists(resourceConfig)).To(BeFalse())
			})
		})
	})
})
