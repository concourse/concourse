package db_test

import (
	"github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkerResourceType", func() {
	var wrt db.WorkerResourceType

	BeforeEach(func() {
		wrt = db.WorkerResourceType{
			Worker:  defaultWorker,
			Image:   "/path/to/image",
			Version: "some-brt-version",
			BaseResourceType: &db.BaseResourceType{
				Name: "some-base-resource-type",
			},
		}
	})

	It("can be find or create worker", func() {
		tx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())
		usedWorkerResourceType, err := wrt.FindOrCreate(tx)
		Expect(err).ToNot(HaveOccurred())
		err = tx.Rollback()
		Expect(err).ToNot(HaveOccurred())

		Expect(usedWorkerResourceType.Worker.Name()).To(Equal(defaultWorker.Name()))
		baseResourceType, found, err := baseResourceTypeFactory.Find("some-base-resource-type")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(usedWorkerResourceType.UsedBaseResourceType.ID).To(Equal(baseResourceType.ID))
	})
})
