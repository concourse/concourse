package db_test

import (
	"github.com/concourse/concourse/atc/db"

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

	Context("when there is a base resource type", func() {
		var unique bool
		var usedWorkerResourceType *db.UsedWorkerResourceType

		BeforeEach(func() {
			unique = false

			tx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			usedWorkerResourceType, err = wrt.FindOrCreate(tx, unique)
			Expect(err).ToNot(HaveOccurred())

			err = tx.Commit()
			Expect(err).ToNot(HaveOccurred())

			Expect(usedWorkerResourceType.Worker.Name()).To(Equal(defaultWorker.Name()))
			Expect(usedWorkerResourceType.UsedBaseResourceType.Name).To(Equal("some-base-resource-type"))
		})

		It("can be found", func() {
			tx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			usedWorkerResourceType2, err := wrt.FindOrCreate(tx, unique)
			Expect(err).ToNot(HaveOccurred())

			err = tx.Commit()
			Expect(err).ToNot(HaveOccurred())

			Expect(usedWorkerResourceType2).To(Equal(usedWorkerResourceType))
			Expect(usedWorkerResourceType2.UsedBaseResourceType.UniqueVersionHistory).To(BeFalse())
		})

		Context("when the base resource type becomes unique", func() {
			BeforeEach(func() {
				unique = true
			})

			It("creates the base resource type with unique history", func() {
				tx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())

				uniqueUsedWorkerResourceType, err := wrt.FindOrCreate(tx, unique)
				Expect(err).ToNot(HaveOccurred())

				err = tx.Commit()
				Expect(err).ToNot(HaveOccurred())

				Expect(uniqueUsedWorkerResourceType).ToNot(Equal(usedWorkerResourceType))
				Expect(uniqueUsedWorkerResourceType.UsedBaseResourceType.UniqueVersionHistory).To(BeTrue())
			})
		})
	})
})
