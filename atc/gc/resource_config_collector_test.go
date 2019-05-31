package gc_test

import (
	"context"
	"time"

	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/creds"
	"github.com/concourse/concourse/v5/atc/db"
	"github.com/concourse/concourse/v5/atc/gc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfigCollector", func() {
	var collector gc.Collector

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
					GraceTime: 1 * time.Minute,
					Min:       5 * time.Minute,
					Max:       10 * time.Minute,
				}

				BeforeEach(func() {
					resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(
						logger,
						"some-base-type",
						atc.Source{
							"some": "source",
						},
						creds.VersionedResourceTypes{},
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

					_, err = worker.CreateContainer(db.NewResourceConfigCheckSessionContainerOwner(resourceConfig, ownerExpiries), db.ContainerMetadata{})
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
					GraceTime: 1 * time.Minute,
					Min:       5 * time.Minute,
					Max:       10 * time.Minute,
				}

				BeforeEach(func() {
					resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(
						logger,
						"some-base-type",
						atc.Source{
							"some": "source",
						},
						creds.VersionedResourceTypes{},
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

					_, err = worker.CreateContainer(db.NewResourceConfigCheckSessionContainerOwner(resourceConfig, ownerExpiries), db.ContainerMetadata{})
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
					Expect(collector.Run(context.TODO())).To(Succeed())
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
					Expect(collector.Run(context.TODO())).To(Succeed())
					Expect(countResourceConfigs()).To(BeZero())
				})
			})

			Context("when config is referenced in resources", func() {
				BeforeEach(func() {
					_, err := usedResource.SetResourceConfig(
						logger,
						atc.Source{"some": "source"},
						creds.VersionedResourceTypes{},
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
						logger,
						"some-base-type",
						atc.Source{"some": "source"},
						creds.VersionedResourceTypes{},
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
						logger,
						atc.Source{"some": "source-type"},
						creds.VersionedResourceTypes{},
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
						logger,
						"some-base-type",
						atc.Source{"some": "source-type"},
						creds.VersionedResourceTypes{},
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
