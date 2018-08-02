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

			resourceConfigCheckSession db.ResourceConfigCheckSession
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

			resourceConfigCheckSession, err = resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(logger,
				defaultWorkerResourceType.Type,
				atc.Source{
					"some-type": "source",
				},
				creds.VersionedResourceTypes{},
				ownerExpiries,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			owner = db.NewResourceConfigCheckSessionContainerOwner(
				resourceConfigCheckSession,
				defaultTeam.ID(),
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

				BeforeEach(func() {
					existingOwner := db.NewResourceConfigCheckSessionContainerOwner(
						resourceConfigCheckSession,
						defaultTeam.ID(),
					)

					tx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					createdColumns, err = existingOwner.Create(tx, worker.Name())
					Expect(err).ToNot(HaveOccurred())
					Expect(createdColumns).ToNot(BeEmpty())

					Expect(tx.Commit()).To(Succeed())
				})

				It("finds the worker resource config check session", func() {
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
