package db_test

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerOwner", func() {

	Describe("ResourceConfigCheckSessionContainerOwner", func() {
		var (
			worker db.Worker

			owner         db.ContainerOwner
			ownerExpiries db.ContainerOwnerExpiries
			found         bool

			resourceConfig      *db.UsedResourceConfig
			otherResourceConfig *db.UsedResourceConfig
		)

		BeforeEach(func() {
			workerPayload := atc.Worker{
				ResourceTypes:   []atc.WorkerResourceType{defaultWorkerResourceType},
				Name:            "resource-config-check-session-worker",
				GardenAddr:      "1.2.3.4:7778",
				BaggageclaimURL: "5.6.7.8:7879",
			}

			var err error
			worker, err = workerFactory.SaveWorker(workerPayload, 0)
			Expect(err).NotTo(HaveOccurred())

			resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger,
				db.ForResource(defaultResource.ID()),
				defaultWorkerResourceType.Type,
				atc.Source{
					"some-type": "source",
				},
				creds.VersionedResourceTypes{},
			)
			Expect(err).ToNot(HaveOccurred())

			otherResourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(logger,
				db.ForResource(defaultResource.ID()),
				defaultWorkerResourceType.Type,
				atc.Source{
					"some-type": "other-source",
				},
				creds.VersionedResourceTypes{},
			)
			Expect(err).ToNot(HaveOccurred())

			ownerExpiries = db.ContainerOwnerExpiries{
				GraceTime: 1 * time.Minute,
				Min:       5 * time.Minute,
				Max:       5 * time.Minute,
			}
		})

		JustBeforeEach(func() {
			owner = db.NewResourceConfigCheckSessionContainerOwner(
				resourceConfig,
				ownerExpiries,
			)
		})

		Describe("Find/Create", func() {
			var foundColumns sq.Eq

			JustBeforeEach(func() {
				var err error
				foundColumns, found, err = owner.Find(dbConn)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when a resource config check session exists", func() {
				var createdColumns map[string]interface{}

				Context("and the GraceTime is 1 minute", func() {
					Context("and it will not expire in the next minute", func() {
						BeforeEach(func() {
							existingOwner := db.NewResourceConfigCheckSessionContainerOwner(
								resourceConfig,
								ownerExpiries,
							)

							tx, err := dbConn.Begin()
							Expect(err).ToNot(HaveOccurred())

							createdColumns, err = existingOwner.Create(tx, worker.Name())
							Expect(err).ToNot(HaveOccurred())
							Expect(createdColumns).ToNot(BeEmpty())

							Expect(tx.Commit()).To(Succeed())
						})

						It("finds the resource config check session", func() {
							Expect(foundColumns).To(BeEquivalentTo(createdColumns))
							Expect(found).To(BeTrue())
						})
					})

					Context("and it expires in 3 seconds", func() {
						BeforeEach(func() {
							ownerExpiries = db.ContainerOwnerExpiries{
								GraceTime: 1 * time.Second,
								Min:       3 * time.Second,
								Max:       3 * time.Second,
							}

							existingOwner := db.NewResourceConfigCheckSessionContainerOwner(
								resourceConfig,
								ownerExpiries,
							)

							tx, err := dbConn.Begin()
							Expect(err).ToNot(HaveOccurred())

							createdColumns, err = existingOwner.Create(tx, worker.Name())
							Expect(err).ToNot(HaveOccurred())
							Expect(createdColumns).ToNot(BeEmpty())

							Expect(tx.Commit()).To(Succeed())
						})

						It("finds a resource config check session within the grace time", func() {
							Expect(foundColumns).To(BeEquivalentTo(createdColumns))
							Expect(found).To(BeTrue())
						})

						Context("when searched within the grace time", func() {
							BeforeEach(func() {
								time.Sleep(2 * time.Second)
							})

							It("doesn't find it", func() {
								Expect(found).To(BeFalse())
							})
						})
					})
				})

				Context("for a different resource config", func() {
					BeforeEach(func() {
						existingOwner := db.NewResourceConfigCheckSessionContainerOwner(
							otherResourceConfig,
							ownerExpiries,
						)

						tx, err := dbConn.Begin()
						Expect(err).ToNot(HaveOccurred())

						createdColumns, err = existingOwner.Create(tx, worker.Name())
						Expect(err).ToNot(HaveOccurred())
						Expect(createdColumns).ToNot(BeEmpty())

						Expect(tx.Commit()).To(Succeed())
					})

					It("doesn't find a resource config check session", func() {
						Expect(found).To(BeFalse())
					})
				})
			})

			Context("when a resource config check session doesn't exist", func() {
				It("doesn't find a resource config check session", func() {
					Expect(found).To(BeFalse())
				})
			})
		})
	})
})
