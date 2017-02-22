package dbng_test

import (
	"github.com/concourse/atc/dbng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkerResourceType", func() {
	var wrt dbng.WorkerResourceType

	BeforeEach(func() {
		wrt = dbng.WorkerResourceType{
			Worker:  defaultWorker,
			Image:   "/path/to/image",
			Version: "some-brt-version",
			BaseResourceType: &dbng.BaseResourceType{
				Name: "some-base-resource-type",
			},
		}
	})

	It("can be find or create worker", func() {
		tx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())
		usedWorkerResourceType, err := wrt.FindOrCreate(tx)
		Expect(err).ToNot(HaveOccurred())
		tx.Rollback()

		Expect(usedWorkerResourceType.Worker.Name()).To(Equal(defaultWorker.Name()))
		baseResourceType, found, err := baseResourceTypeFactory.Find("some-base-resource-type")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(usedWorkerResourceType.UsedBaseResourceType.ID).To(Equal(baseResourceType.ID))
	})
})
