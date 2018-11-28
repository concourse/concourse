package db_test

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"

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

			resourceConfig db.ResourceConfig
		)

		ownerExpiries = db.ContainerOwnerExpiries{
			GraceTime: 1 * time.Minute,
			Min:       5 * time.Minute,
			Max:       5 * time.Minute,
		}

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
				defaultWorkerResourceType.Type,
				atc.Source{
					"some-type": "source",
				},
				creds.VersionedResourceTypes{},
			)
			Expect(err).ToNot(HaveOccurred())
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

			Context("when a resource config exists", func() {
				var createdColumns map[string]interface{}

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

			Context("when a resource config check session doesn't exist", func() {
				It("doesn't find a resource config check session", func() {
					Expect(found).To(BeFalse())
				})
			})
		})
	})
})
