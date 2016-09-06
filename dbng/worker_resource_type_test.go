package dbng_test

import (
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// import (
// 	. "github.com/concourse/atc/dbng"

// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"
// )

// var _ = Describe("WorkerResourceType", func() {

// })
var _ = Describe("WorkerResourceType", func() {
	var dbConn dbng.Conn
	var tx dbng.Tx

	var wrt dbng.WorkerResourceType

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = dbng.Wrap(postgresRunner.Open())

		worker := dbng.Worker{
			Name:       "some-worker",
			GardenAddr: "1.2.3.4:7777",
		}

		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		defer setupTx.Rollback()

		err = worker.Create(setupTx)
		Expect(err).ToNot(HaveOccurred())

		Expect(setupTx.Commit()).To(Succeed())

		tx, err = dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		wrt = dbng.WorkerResourceType{
			WorkerName: worker.Name,
			Type:       "some-worker-resource-type",
			Image:      "some-worker-resource-image",
			Version:    "some-worker-resource-version",
		}
	})

	AfterEach(func() {
		err := tx.Rollback()
		Expect(err).NotTo(HaveOccurred())

		err = dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	It("can be created and looked up", func() {
		foundID, found, err := wrt.Lookup(tx)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(foundID).To(BeZero())

		createdID, err := wrt.Create(tx)
		Expect(err).ToNot(HaveOccurred())
		Expect(createdID).ToNot(BeZero())

		foundID, found, err = wrt.Lookup(tx)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(foundID).To(Equal(createdID))
	})

	// TODO: this should maybe be idempotent instead
	Context("when it already exists", func() {
		BeforeEach(func() {
			_, err := wrt.Create(tx)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns ErrCacheAlreadyExists", func() {
			id, err := wrt.Create(tx)
			Expect(err).To(Equal(dbng.ErrWorkerResourceTypeAlreadyExists))
			Expect(id).To(BeZero())
		})
	})

	Context("when another version already exists", func() {
		var otherWRT dbng.WorkerResourceType

		BeforeEach(func() {
			otherWRT = wrt
			otherWRT.Version = "older-version"

			_, err := otherWRT.Create(tx)
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates the new one and removes the old one", func() {
			createdID, err := wrt.Create(tx)
			Expect(err).ToNot(HaveOccurred())
			Expect(createdID).ToNot(BeZero())

			_, found, err := otherWRT.Lookup(tx)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})
})
