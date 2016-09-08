package dbng_test

// import (
// 	"github.com/concourse/atc/dbng"

// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"
// )

// var _ = Describe("VolumeFactory", func() {
// 	var dbConn dbng.Conn

// 	var factory *dbng.VolumeFactory

// 	BeforeEach(func() {
// 		postgresRunner.Truncate()

// 		dbConn = dbng.Wrap(postgresRunner.Open())

// 		factory = dbng.NewVolumeFactory(dbConn)
// 	})

// 	AfterEach(func() {
// 		err := dbConn.Close()
// 		Expect(err).NotTo(HaveOccurred())
// 	})

// 	Describe("CreateWorkerResourceTypeVolume", func() {
// 		var worker dbng.Worker
// 		var wrt dbng.WorkerResourceType

// 		BeforeEach(func() {
// 			worker = dbng.Worker{
// 				Name:       "some-worker",
// 				GardenAddr: "1.2.3.4:7777",
// 			}

// 			wrt = dbng.WorkerResourceType{
// 				WorkerName: worker.Name,
// 				Type:       "some-worker-resource-type",
// 				Image:      "some-worker-resource-image",
// 				Version:    "some-worker-resource-version",
// 			}
// 		})

// 		Context("when the worker resource type exists", func() {
// 			BeforeEach(func() {
// 				setupTx, err := dbConn.Begin()
// 				Expect(err).ToNot(HaveOccurred())

// 				defer setupTx.Rollback()

// 				err = worker.Create(setupTx)
// 				Expect(err).ToNot(HaveOccurred())

// 				_, err = wrt.Create(setupTx)
// 				Expect(err).ToNot(HaveOccurred())

// 				Expect(setupTx.Commit()).To(Succeed())
// 			})

// 			It("returns the created volume", func() {
// 				volume, err := factory.CreateWorkerResourceTypeVolume(wrt)
// 				Expect(err).ToNot(HaveOccurred())
// 				Expect(volume.ID).ToNot(BeZero())
// 			})
// 		})

// 		Context("when the worker resource type does not exist", func() {
// 			It("returns ErrWorkerResourceTypeNotFound", func() {
// 				_, err := factory.CreateWorkerResourceTypeVolume(wrt)
// 				Expect(err).To(Equal(dbng.ErrWorkerResourceTypeNotFound))
// 			})
// 		})
// 	})
// })
