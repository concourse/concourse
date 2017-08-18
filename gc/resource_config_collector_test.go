package gc_test

import (
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/gc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfigCollector", func() {
	var collector gc.Collector

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("resource-cache-use-collector")
		collector = gc.NewResourceConfigCollector(logger, resourceConfigFactory)
	})

	Describe("Run", func() {
		Describe("configs", func() {
			countResourceConfigs := func() int {
				tx, err := dbConn.Begin()
				Expect(err).NotTo(HaveOccurred())
				defer tx.Rollback()

				var result int
				err = psql.Select("count(*)").
					From("resource_configs").
					RunWith(tx).
					QueryRow().
					Scan(&result)
				Expect(err).NotTo(HaveOccurred())

				return result
			}

			Context("when the config is referenced in resource config check sessions", func() {
				ownerExpiries := db.ContainerOwnerExpiries{
					GraceTime: 1 * time.Minute,
					Min:       5 * time.Minute,
					Max:       10 * time.Minute,
				}

				BeforeEach(func() {
					_, err = resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(
						logger,
						"some-base-type",
						atc.Source{
							"some": "source",
						},
						creds.VersionedResourceTypes{},
						ownerExpiries,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("preserves the config", func() {
					Expect(countResourceConfigs()).ToNot(BeZero())
					Expect(collector.Run()).To(Succeed())
					Expect(countResourceConfigs()).ToNot(BeZero())
				})
			})

			Context("when the config is no longer referenced in resource config check sessions", func() {
				ownerExpiries := db.ContainerOwnerExpiries{
					GraceTime: 1 * time.Minute,
					Min:       5 * time.Minute,
					Max:       10 * time.Minute,
				}

				BeforeEach(func() {
					_, err = resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(
						logger,
						"some-base-type",
						atc.Source{
							"some": "source",
						},
						creds.VersionedResourceTypes{},
						ownerExpiries,
					)
					Expect(err).NotTo(HaveOccurred())

					tx, err := dbConn.Begin()
					Expect(err).NotTo(HaveOccurred())
					defer tx.Rollback()
					_, err = psql.Delete("resource_config_check_sessions").
						RunWith(tx).Exec()
					Expect(err).NotTo(HaveOccurred())
					Expect(tx.Commit()).To(Succeed())
				})

				It("cleans up the config", func() {
					Expect(countResourceConfigs()).NotTo(BeZero())
					Expect(collector.Run()).To(Succeed())
					Expect(countResourceConfigs()).To(BeZero())
				})
			})

			Context("when config is referenced in resource caches", func() {
				BeforeEach(func() {
					_, err = resourceCacheFactory.FindOrCreateResourceCache(
						logger,
						db.ForBuild(defaultBuild.ID()),
						"some-base-type",
						atc.Version{"some": "version"},
						atc.Source{
							"some": "source",
						},
						nil,
						creds.VersionedResourceTypes{},
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("preserve the config", func() {
					Expect(countResourceConfigs()).NotTo(BeZero())
					Expect(collector.Run()).To(Succeed())
					Expect(countResourceConfigs()).NotTo(BeZero())
				})
			})

			Context("when config is not referenced in resource caches", func() {
				BeforeEach(func() {
					_, err = resourceCacheFactory.FindOrCreateResourceCache(
						logger,
						db.ForBuild(defaultBuild.ID()),
						"some-base-type",
						atc.Version{"some": "version"},
						atc.Source{
							"some": "source",
						},
						nil,
						creds.VersionedResourceTypes{},
					)
					Expect(err).NotTo(HaveOccurred())

					tx, err := dbConn.Begin()
					Expect(err).NotTo(HaveOccurred())
					defer tx.Rollback()
					_, err = psql.Delete("resource_cache_uses").
						RunWith(tx).Exec()
					_, err = psql.Delete("resource_caches").
						RunWith(tx).Exec()
					Expect(err).NotTo(HaveOccurred())
					Expect(tx.Commit()).To(Succeed())
				})

				It("cleans up the config", func() {
					Expect(countResourceConfigs()).NotTo(BeZero())
					Expect(collector.Run()).To(Succeed())
					Expect(countResourceConfigs()).To(BeZero())
				})
			})
		})
	})
})
