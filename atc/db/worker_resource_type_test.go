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

	It("can be found/created", func() {
		tx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		usedWorkerResourceType, err := wrt.FindOrCreate(tx)
		Expect(err).ToNot(HaveOccurred())

		err = tx.Commit()
		Expect(err).ToNot(HaveOccurred())

		Expect(usedWorkerResourceType.Worker.Name()).To(Equal(defaultWorker.Name()))
		Expect(usedWorkerResourceType.UsedBaseResourceType.Name).To(Equal("some-base-resource-type"))

		tx, err = dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())

		usedWorkerResourceType2, err := wrt.FindOrCreate(tx)
		Expect(err).ToNot(HaveOccurred())

		err = tx.Commit()
		Expect(err).ToNot(HaveOccurred())

		Expect(usedWorkerResourceType2).To(Equal(usedWorkerResourceType))
	})
})
