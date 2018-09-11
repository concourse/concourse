package db_test

import (
	"strconv"
	"time"

	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfig", func() {
	Describe("AcquireResourceConfigCheckingLockWithIntervalCheck", func() {
		var (
			someResource               db.Resource
			resourceConfigCheckSession db.ResourceConfigCheckSession
			resourceConfig             db.ResourceConfig
		)

		ownerExpiries := db.ContainerOwnerExpiries{
			GraceTime: 1 * time.Minute,
			Min:       5 * time.Minute,
			Max:       5 * time.Minute,
		}

		BeforeEach(func() {
			var err error
			var found bool

			resourceConfigFactory = db.NewResourceConfigFactory(dbConn, lockFactory)
			someResource, found, err = defaultPipeline.Resource("some-resource")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			pipelineResourceTypes, err := defaultPipeline.ResourceTypes()
			Expect(err).ToNot(HaveOccurred())

			resourceConfigCheckSession, err = resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(
				logger,
				someResource.Type(),
				someResource.Source(),
				creds.NewVersionedResourceTypes(template.StaticVariables{}, pipelineResourceTypes.Deserialize()),
				ownerExpiries,
			)
			Expect(err).ToNot(HaveOccurred())

			resourceConfig = resourceConfigCheckSession.ResourceConfig()
		})

		Context("when there has been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets the lock", func() {
					lock, acquired, err := resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, false)
					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeTrue())

					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())

					lock, acquired, err = resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, true)
					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeTrue())

					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when not acquiring immediately", func() {
				It("does not get the lock", func() {
					lock, acquired, err := resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, false)
					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeTrue())

					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())

					_, acquired, err = resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, false)
					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeFalse())
				})
			})
		})

		Context("when there has not been a check recently", func() {
			Context("when acquiring immediately", func() {
				It("gets and keeps the lock and stops others from periodically getting it", func() {
					lock, acquired, err := resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, true)
					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, false)
						Expect(err).ToNot(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())

					time.Sleep(time.Second)

					lock, acquired, err = resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, true)
					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeTrue())

					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())
				})

				It("gets and keeps the lock and stops others from immediately getting it", func() {
					lock, acquired, err := resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, true)
					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, true)
						Expect(err).ToNot(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())

					time.Sleep(time.Second)

					lock, acquired, err = resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, true)
					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeTrue())

					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when not acquiring immediately", func() {
				It("gets and keeps the lock and stops others from periodically getting it", func() {
					lock, acquired, err := resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, false)
					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, false)
						Expect(err).ToNot(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())

					time.Sleep(time.Second)

					lock, acquired, err = resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, false)
					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeTrue())

					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())
				})

				It("gets and keeps the lock and stops others from immediately getting it", func() {
					lock, acquired, err := resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, false)

					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeTrue())

					Consistently(func() bool {
						_, acquired, err = resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, true)
						Expect(err).ToNot(HaveOccurred())

						return acquired
					}, 1500*time.Millisecond, 100*time.Millisecond).Should(BeFalse())

					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())

					time.Sleep(time.Second)

					lock, acquired, err = resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(logger, 1*time.Second, false)
					Expect(err).ToNot(HaveOccurred())
					Expect(acquired).To(BeTrue())

					err = lock.Release()
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})
	})

	Describe("SaveResourceVersions", func() {
		var (
			originalVersionSlice []atc.Version
			resourceConfig       db.ResourceConfig
		)

		BeforeEach(func() {
			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "some-type",
			}
			_, err = brt.FindOrCreate(setupTx)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceConfigFactory := db.NewResourceConfigFactory(dbConn, lockFactory)
			resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"source-config": "some-value"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			originalVersionSlice = []atc.Version{
				{"ref": "v1"},
				{"ref": "v3"},
			}
		})

		// XXX: Can make test more resilient if there is a method that gives all versions by descending check order
		It("ensures versioned resources have the correct check_order", func() {
			err := resourceConfig.SaveVersions(originalVersionSlice)
			Expect(err).ToNot(HaveOccurred())

			latestVR, found, err := resourceConfig.LatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version()).To(Equal(db.Version{"ref": "v3"}))
			Expect(latestVR.CheckOrder()).To(Equal(2))

			pretendCheckResults := []atc.Version{
				{"ref": "v2"},
				{"ref": "v3"},
			}

			err = resourceConfig.SaveVersions(pretendCheckResults)
			Expect(err).ToNot(HaveOccurred())

			latestVR, found, err = resourceConfig.LatestVersion()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(latestVR.Version()).To(Equal(db.Version{"ref": "v3"}))
			Expect(latestVR.CheckOrder()).To(Equal(4))
		})

		Context("when the versions already exists", func() {
			var newVersionSlice []atc.Version

			BeforeEach(func() {
				newVersionSlice = []atc.Version{
					{"ref": "v1"},
					{"ref": "v3"},
				}
			})

			It("does not change the check order", func() {
				err := resourceConfig.SaveVersions(newVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				latestVR, found, err := resourceConfig.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(latestVR.Version()).To(Equal(db.Version{"ref": "v3"}))
				Expect(latestVR.CheckOrder()).To(Equal(2))
			})
		})
	})

	Describe("LatestVersion", func() {
		var (
			originalVersionSlice []atc.Version
			resourceConfig       db.ResourceConfig
			latestCV             db.ResourceConfigVersion
			found                bool
		)

		Context("when the resource config exists", func() {
			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "some-type",
				}
				_, err = brt.FindOrCreate(setupTx)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				resourceConfigFactory := db.NewResourceConfigFactory(dbConn, lockFactory)
				resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"source-config": "some-value"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				originalVersionSlice = []atc.Version{
					{"ref": "v1"},
					{"ref": "v3"},
				}

				err = resourceConfig.SaveVersions(originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				latestCV, found, err = resourceConfig.LatestVersion()
				Expect(err).ToNot(HaveOccurred())
			})

			It("gets latest version of resource", func() {
				Expect(found).To(BeTrue())

				Expect(latestCV.Version()).To(Equal(db.Version{"ref": "v3"}))
				Expect(latestCV.CheckOrder()).To(Equal(2))
			})
		})
	})

	Describe("FindVersion", func() {
		var (
			originalVersionSlice []atc.Version
			resourceConfig       db.ResourceConfig
			latestCV             db.ResourceConfigVersion
			found                bool
		)

		BeforeEach(func() {
			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "some-type",
			}
			_, err = brt.FindOrCreate(setupTx)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"source-config": "some-value"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			originalVersionSlice = []atc.Version{
				{"ref": "v1"},
				{"ref": "v3"},
			}

			err = resourceConfig.SaveVersions(originalVersionSlice)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the version exists", func() {
			BeforeEach(func() {
				var err error
				latestCV, found, err = resourceConfig.FindVersion(atc.Version{"ref": "v1"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("gets the version of resource", func() {
				Expect(found).To(BeTrue())

				Expect(latestCV.ResourceConfig().ID()).To(Equal(resourceConfig.ID()))
				Expect(latestCV.Version()).To(Equal(db.Version{"ref": "v1"}))
				Expect(latestCV.CheckOrder()).To(Equal(1))
			})
		})

		Context("when the version does not exist", func() {
			BeforeEach(func() {
				var err error
				latestCV, found, err = resourceConfig.FindVersion(atc.Version{"ref": "v2"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not get the version of resource", func() {
				Expect(found).To(BeFalse())
			})
		})
	})

	Context("Versions", func() {
		var (
			originalVersionSlice []atc.Version
			resourceConfig       db.ResourceConfig
		)

		Context("when resource has versions created in order of check order", func() {
			var resourceConfigVersions db.ResourceConfigVersions

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "some-type",
				}
				_, err = brt.FindOrCreate(setupTx)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				originalVersionSlice = []atc.Version{
					{"ref": "v0"},
					{"ref": "v1"},
					{"ref": "v2"},
					{"ref": "v3"},
					{"ref": "v4"},
					{"ref": "v5"},
					{"ref": "v6"},
					{"ref": "v7"},
					{"ref": "v8"},
					{"ref": "v9"},
				}

				err = resourceConfig.SaveVersions(originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				resourceConfigVersions = nil
				for i := 0; i < 10; i++ {
					rcv, found, err := resourceConfig.FindVersion(atc.Version{"ref": "v" + strconv.Itoa(i)})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					resourceConfigVersions = append(resourceConfigVersions, rcv)
				}
			})

			Context("with no since/until", func() {
				It("returns the first page, with the given limit, and a next page", func() {
					historyPage, pagination, found, err := resourceConfig.Versions(db.Page{Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version()).To(Equal(resourceConfigVersions[9].Version()))
					Expect(historyPage[1].Version()).To(Equal(resourceConfigVersions[8].Version()))
					Expect(pagination.Previous).To(BeNil())
					Expect(pagination.Next).To(Equal(&db.Page{Since: resourceConfigVersions[8].ID(), Limit: 2}))
				})
			})

			Context("with a since that places it in the middle of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := resourceConfig.Versions(db.Page{Since: resourceConfigVersions[6].ID(), Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version()).To(Equal(resourceConfigVersions[5].Version()))
					Expect(historyPage[1].Version()).To(Equal(resourceConfigVersions[4].Version()))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: resourceConfigVersions[5].ID(), Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: resourceConfigVersions[4].ID(), Limit: 2}))
				})
			})

			Context("with a since that places it at the end of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := resourceConfig.Versions(db.Page{Since: resourceConfigVersions[2].ID(), Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version()).To(Equal(resourceConfigVersions[1].Version()))
					Expect(historyPage[1].Version()).To(Equal(resourceConfigVersions[0].Version()))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: resourceConfigVersions[1].ID(), Limit: 2}))
					Expect(pagination.Next).To(BeNil())
				})
			})

			Context("with an until that places it in the middle of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := resourceConfig.Versions(db.Page{Until: resourceConfigVersions[6].ID(), Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version()).To(Equal(resourceConfigVersions[8].Version()))
					Expect(historyPage[1].Version()).To(Equal(resourceConfigVersions[7].Version()))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: resourceConfigVersions[8].ID(), Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: resourceConfigVersions[7].ID(), Limit: 2}))
				})
			})

			Context("with a until that places it at the beginning of the builds", func() {
				It("returns the builds, with previous/next pages", func() {
					historyPage, pagination, found, err := resourceConfig.Versions(db.Page{Until: resourceConfigVersions[7].ID(), Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(2))
					Expect(historyPage[0].Version()).To(Equal(resourceConfigVersions[9].Version()))
					Expect(historyPage[1].Version()).To(Equal(resourceConfigVersions[8].Version()))
					Expect(pagination.Previous).To(BeNil())
					Expect(pagination.Next).To(Equal(&db.Page{Since: resourceConfigVersions[8].ID(), Limit: 2}))
				})
			})

			Context("when the version has metadata", func() {
				BeforeEach(func() {
					metadata := []db.ResourceConfigMetadataField{{Name: "name1", Value: "value1"}}

					// save metadata
					_, err := resourceConfig.SaveVersion(atc.Version(resourceConfigVersions[9].Version()), metadata)
					Expect(err).ToNot(HaveOccurred())

					reloaded, err := resourceConfigVersions[9].Reload()
					Expect(reloaded).To(BeTrue())
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns the metadata in the version history", func() {
					historyPage, _, found, err := resourceConfig.Versions(db.Page{Limit: 1})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(len(historyPage)).To(Equal(1))
					Expect(historyPage[0].Version()).To(Equal(resourceConfigVersions[9].Version()))
				})
			})

			Context("when a version is disabled", func() {
				BeforeEach(func() {
					pipeline, created, err := defaultTeam.SavePipeline("new-pipeline", atc.Config{
						Resources: atc.ResourceConfigs{
							{
								Name:   "some-resource",
								Type:   "some-type",
								Source: atc.Source{"some": "source"},
							},
						},
					}, db.ConfigVersion(0), db.PipelineUnpaused)
					Expect(err).ToNot(HaveOccurred())
					Expect(created).To(BeTrue())

					resource, found, err := pipeline.Resource("some-resource")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					err = pipeline.DisableResourceVersion(resource.ID(), resourceConfigVersions[9].ID())
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns a disabled version", func() {
					historyPage, _, found, err := resourceConfig.Versions(db.Page{Limit: 1})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(ConsistOf(db.ResourceConfigVersions{resourceConfigVersions[9]}))
				})
			})
		})

		Context("when check orders are different than versions ids", func() {
			var resourceConfigVersions db.ResourceConfigVersions

			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "some-type",
				}
				_, err = brt.FindOrCreate(setupTx)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				resourceConfigFactory := db.NewResourceConfigFactory(dbConn, lockFactory)
				resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				originalVersionSlice := []atc.Version{
					{"ref": "v1"}, // id: 1, check_order: 1
					{"ref": "v3"}, // id: 2, check_order: 2
					{"ref": "v4"}, // id: 3, check_order: 3
				}

				err = resourceConfig.SaveVersions(originalVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				secondVersionSlice := []atc.Version{
					{"ref": "v2"}, // id: 4, check_order: 4
					{"ref": "v3"}, // id: 2, check_order: 5
					{"ref": "v4"}, // id: 3, check_order: 6
				}

				err = resourceConfig.SaveVersions(secondVersionSlice)
				Expect(err).ToNot(HaveOccurred())

				for i := 1; i < 5; i++ {
					rcv, found, err := resourceConfig.FindVersion(atc.Version{"ref": "v" + strconv.Itoa(i)})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					resourceConfigVersions = append(resourceConfigVersions, rcv)
				}

				// ids ordered by check order now: [3, 2, 4, 1]
			})

			Context("with no since/until", func() {
				It("returns versions ordered by check order", func() {
					historyPage, pagination, found, err := resourceConfig.Versions(db.Page{Limit: 4})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(4))
					Expect(historyPage[0].Version()).To(Equal(resourceConfigVersions[3].Version()))
					Expect(historyPage[1].Version()).To(Equal(resourceConfigVersions[2].Version()))
					Expect(historyPage[2].Version()).To(Equal(resourceConfigVersions[1].Version()))
					Expect(historyPage[3].Version()).To(Equal(resourceConfigVersions[0].Version()))
					Expect(pagination.Previous).To(BeNil())
					Expect(pagination.Next).To(BeNil())
				})
			})

			Context("with a since", func() {
				It("returns the builds, with previous/next pages excluding since", func() {
					historyPage, pagination, found, err := resourceConfig.Versions(db.Page{Since: 3, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage[0].Version()).To(Equal(resourceConfigVersions[2].Version()))
					Expect(historyPage[1].Version()).To(Equal(resourceConfigVersions[1].Version()))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: 4, Limit: 2}))
				})
			})

			Context("with from", func() {
				It("returns the builds, with previous/next pages including from", func() {
					historyPage, pagination, found, err := resourceConfig.Versions(db.Page{From: 2, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage[0].Version()).To(Equal(resourceConfigVersions[2].Version()))
					Expect(historyPage[1].Version()).To(Equal(resourceConfigVersions[1].Version()))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: 4, Limit: 2}))
				})
			})

			Context("with a until", func() {
				It("returns the builds, with previous/next pages excluding until", func() {
					historyPage, pagination, found, err := resourceConfig.Versions(db.Page{Until: 1, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage[0].Version()).To(Equal(resourceConfigVersions[2].Version()))
					Expect(historyPage[1].Version()).To(Equal(resourceConfigVersions[1].Version()))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: 4, Limit: 2}))
				})
			})

			Context("with to", func() {
				It("returns the builds, with previous/next pages including to", func() {
					historyPage, pagination, found, err := resourceConfig.Versions(db.Page{To: 4, Limit: 2})
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(historyPage).To(HaveLen(2))
					Expect(historyPage[0].Version()).To(Equal(resourceConfigVersions[2].Version()))
					Expect(historyPage[1].Version()).To(Equal(resourceConfigVersions[1].Version()))
					Expect(pagination.Previous).To(Equal(&db.Page{Until: 2, Limit: 2}))
					Expect(pagination.Next).To(Equal(&db.Page{Since: 4, Limit: 2}))
				})
			})
		})

		Context("when resource has a version with check order of 0", func() {
			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				brt := db.BaseResourceType{
					Name: "some-type",
				}
				_, err = brt.FindOrCreate(setupTx)
				Expect(err).NotTo(HaveOccurred())
				Expect(setupTx.Commit()).To(Succeed())

				resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
				Expect(err).ToNot(HaveOccurred())

				created, err := resourceConfig.SaveVersion(atc.Version{"version": "not-returned"}, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(created).To(BeTrue())
			})

			It("does not return the version", func() {
				historyPage, pagination, found, err := resourceConfig.Versions(db.Page{Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(historyPage).To(Equal(db.ResourceConfigVersions{}))
				Expect(pagination).To(Equal(db.Pagination{Previous: nil, Next: nil}))
			})
		})
	})
})
