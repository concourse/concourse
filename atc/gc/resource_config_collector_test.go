package gc_test

import (
	"context"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/gc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfigCollector", func() {
	var collector GcCollector

	BeforeEach(func() {
		collector = gc.NewResourceConfigCollector(resourceConfigFactory)
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
					Min: 5 * time.Minute,
					Max: 10 * time.Minute,
				}

				BeforeEach(func() {
					resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(
						"some-base-type",
						atc.Source{
							"some": "source",
						},
						atc.VersionedResourceTypes{},
					)
					Expect(err).NotTo(HaveOccurred())

					workerFactory := db.NewWorkerFactory(dbConn)
					defaultWorkerPayload := atc.Worker{
						ResourceTypes: []atc.WorkerResourceType{
							atc.WorkerResourceType{
								Type:    "some-base-type",
								Image:   "/path/to/image",
								Version: "some-brt-version",
							},
						},
						Name:            "default-worker",
						GardenAddr:      "1.2.3.4:7777",
						BaggageclaimURL: "5.6.7.8:7878",
					}
					worker, err := workerFactory.SaveWorker(defaultWorkerPayload, 0)
					Expect(err).NotTo(HaveOccurred())

					_, err = worker.CreateContainer(db.NewResourceConfigCheckSessionContainerOwner(resourceConfig.ID(), resourceConfig.OriginBaseResourceType().ID, ownerExpiries), db.ContainerMetadata{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("preserves the config", func() {
					Expect(countResourceConfigs()).ToNot(BeZero())
					Expect(collector.Run(context.TODO())).To(Succeed())
					Expect(countResourceConfigs()).ToNot(BeZero())
				})
			})

			Context("when the config is no longer referenced in resource config check sessions", func() {
				ownerExpiries := db.ContainerOwnerExpiries{
					Min: 5 * time.Minute,
					Max: 10 * time.Minute,
				}

				BeforeEach(func() {
					resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(
						"some-base-type",
						atc.Source{
							"some": "source",
						},
						atc.VersionedResourceTypes{},
					)
					Expect(err).NotTo(HaveOccurred())

					workerFactory := db.NewWorkerFactory(dbConn)
					defaultWorkerPayload := atc.Worker{
						ResourceTypes: []atc.WorkerResourceType{
							atc.WorkerResourceType{
								Type:    "some-base-type",
								Image:   "/path/to/image",
								Version: "some-brt-version",
							},
						},
						Name:            "default-worker",
						GardenAddr:      "1.2.3.4:7777",
						BaggageclaimURL: "5.6.7.8:7878",
					}
					worker, err := workerFactory.SaveWorker(defaultWorkerPayload, 0)
					Expect(err).NotTo(HaveOccurred())

					_, err = worker.CreateContainer(db.NewResourceConfigCheckSessionContainerOwner(resourceConfig.ID(), resourceConfig.OriginBaseResourceType().ID, ownerExpiries), db.ContainerMetadata{})
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
					Expect(collector.Run(context.TODO())).To(Succeed())
					Expect(countResourceConfigs()).To(BeZero())
				})
			})

			Context("when config is referenced in resource caches", func() {
				BeforeEach(func() {
					_, err = resourceCacheFactory.FindOrCreateResourceCache(
						db.ForBuild(defaultBuild.ID()),
						"some-base-type",
						atc.Version{"some": "version"},
						atc.Source{
							"some": "source",
						},
						nil,
						atc.VersionedResourceTypes{},
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("preserve the config", func() {
					Expect(countResourceConfigs()).NotTo(BeZero())
					Expect(collector.Run(context.TODO())).To(Succeed())
					Expect(countResourceConfigs()).NotTo(BeZero())
				})
			})

			Context("when config is not referenced in resource caches", func() {
				BeforeEach(func() {
					_, err = resourceCacheFactory.FindOrCreateResourceCache(
						db.ForBuild(defaultBuild.ID()),
						"some-base-type",
						atc.Version{"some": "version"},
						atc.Source{
							"some": "source",
						},
						nil,
						atc.VersionedResourceTypes{},
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
					Expect(collector.Run(context.TODO())).To(Succeed())
					Expect(countResourceConfigs()).To(BeZero())
				})
			})

			Context("when config is referenced in resources", func() {
				BeforeEach(func() {
					_, err := usedResource.SetResourceConfig(
						atc.Source{"some": "source"},
						atc.VersionedResourceTypes{},
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("preserve the config", func() {
					Expect(countResourceConfigs()).NotTo(BeZero())
					Expect(collector.Run(context.TODO())).To(Succeed())
					Expect(countResourceConfigs()).NotTo(BeZero())
				})
			})

			Context("when config is not referenced in resources", func() {
				BeforeEach(func() {
					_, err := resourceConfigFactory.FindOrCreateResourceConfig(
						"some-base-type",
						atc.Source{"some": "source"},
						atc.VersionedResourceTypes{},
					)
					Expect(err).NotTo(HaveOccurred())
					_, err = usedResource.Reload()
					Expect(err).NotTo(HaveOccurred())
					Expect(usedResource.ResourceConfigID()).To(BeZero())
				})

				It("cleans up the config", func() {
					Expect(countResourceConfigs()).NotTo(BeZero())
					Expect(collector.Run(context.TODO())).To(Succeed())
					Expect(countResourceConfigs()).To(BeZero())
				})
			})

			Context("when config is referenced in resource types", func() {
				BeforeEach(func() {
					_, err := usedResourceType.SetResourceConfig(
						atc.Source{"some": "source-type"},
						atc.VersionedResourceTypes{},
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("preserve the config", func() {
					Expect(countResourceConfigs()).NotTo(BeZero())
					Expect(collector.Run(context.TODO())).To(Succeed())
					Expect(countResourceConfigs()).NotTo(BeZero())
				})
			})

			Context("when config is not referenced in resource types", func() {
				BeforeEach(func() {
					_, err := resourceConfigFactory.FindOrCreateResourceConfig(
						"some-base-type",
						atc.Source{"some": "source-type"},
						atc.VersionedResourceTypes{},
					)
					Expect(err).NotTo(HaveOccurred())
					_, err = usedResourceType.Reload()
					Expect(err).NotTo(HaveOccurred())
				})

				It("cleans up the config", func() {
					Expect(countResourceConfigs()).NotTo(BeZero())
					Expect(collector.Run(context.TODO())).To(Succeed())
					Expect(countResourceConfigs()).To(BeZero())
				})
			})
		})
	})
})
