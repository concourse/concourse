package db_test

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"

	. "github.com/onsi/ginkgo/v2"
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
			Min: 5 * time.Minute,
			Max: 5 * time.Minute,
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

			resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(
				defaultWorkerResourceType.Type,
				atc.Source{
					"some-type": "source",
				},
				nil,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			owner = db.NewResourceConfigCheckSessionContainerOwner(
				resourceConfig.ID(),
				resourceConfig.OriginBaseResourceType().ID,
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
						resourceConfig.ID(),
						resourceConfig.OriginBaseResourceType().ID,
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
					Expect(foundColumns).To(HaveLen(1))
					Expect(foundColumns["resource_config_check_session_id"]).To(ConsistOf(createdColumns["resource_config_check_session_id"]))
					Expect(found).To(BeTrue())
				})
			})

			Context("when there are multiple resource config check sessions", func() {
				var createdColumns, createdColumns2 map[string]interface{}

				BeforeEach(func() {
					existingOwner := db.NewResourceConfigCheckSessionContainerOwner(
						resourceConfig.ID(),
						resourceConfig.OriginBaseResourceType().ID,
						ownerExpiries,
					)

					tx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())

					createdColumns, err = existingOwner.Create(tx, worker.Name())
					Expect(err).ToNot(HaveOccurred())
					Expect(createdColumns).ToNot(BeEmpty())

					createdColumns2, err = existingOwner.Create(tx, defaultWorker.Name())
					Expect(err).ToNot(HaveOccurred())
					Expect(createdColumns).ToNot(BeEmpty())

					Expect(tx.Commit()).To(Succeed())
				})

				It("finds both resource config check sessions", func() {
					Expect(foundColumns).To(HaveLen(1))
					Expect(foundColumns["resource_config_check_session_id"]).To(ConsistOf(createdColumns["resource_config_check_session_id"], createdColumns2["resource_config_check_session_id"]))
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
